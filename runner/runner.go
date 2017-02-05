package runner

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/urfave/cli"
)

var Command = cli.Command{
	Name:   "run",
	Hidden: true,
	Action: run,
}

func run(c *cli.Context) error {
	args := c.Args()
	if len(args) < 3 {
		return errors.New("a directory, log file and command is required")
	}
	workingDir := os.ExpandEnv(args[0])
	logFile := os.ExpandEnv(args[1])
	fullCommand := os.ExpandEnv(args[2])

	command, cmdArgs, err := ParseCommand(fullCommand)
	if err != nil {
		return errors.WithStack(err)
	}

	log, err := os.Create(logFile)
	if err != nil {
		return errors.WithStack(err)
	}

	standardLog := &RunnerLog{
		file:   log,
		name:   fullCommand,
		stream: "stdout",
	}
	errorLog := &RunnerLog{
		file:   log,
		name:   fullCommand,
		stream: "stderr",
	}

	cmd := exec.Command(command, cmdArgs...)
	fmt.Println(cmd.Path)
	cmd.Dir = workingDir
	cmd.Stdout = standardLog
	cmd.Stderr = errorLog
	return cmd.Run()
}

// RunnerLog provides the io.Writer interface to publish service logs to file
type RunnerLog struct {
	file   *os.File
	name   string
	stream string
}

func (r *RunnerLog) Write(p []byte) (int, error) {
	lineData := LogLine{
		Name:    r.name,
		Time:    time.Now(),
		Stream:  r.stream,
		Message: strings.TrimSpace(string(p)),
	}

	jsonContent, err := json.Marshal(lineData)
	if err != nil {
		return 0, errors.Wrap(err, "could not prepare log line")
	}

	line := fmt.Sprintln(string(jsonContent))
	count, err := r.file.Write([]byte(line))
	if err != nil {
		fmt.Println("Error")
		return count, errors.Wrap(err, "could not write log line")
	}
	return len(p), nil
}

// Returns the executable path and arguments
// TODO: Clean this up
func ParseCommand(cmd string) (string, []string, error) {
	var args []string
	state := "start"
	current := ""
	quote := "\""
	for i := 0; i < len(cmd); i++ {
		c := cmd[i]

		if state == "quotes" {
			if string(c) != quote {
				current += string(c)
			} else {
				args = append(args, current)
				current = ""
				state = "start"
			}
			continue
		}

		if c == '"' || c == '\'' {
			state = "quotes"
			quote = string(c)
			continue
		}

		if state == "arg" {
			if c == ' ' || c == '\t' {
				args = append(args, current)
				current = ""
				state = "start"
			} else {
				current += string(c)
			}
			continue
		}

		if c != ' ' && c != '\t' {
			state = "arg"
			current += string(c)
		}
	}

	if state == "quotes" {
		return "", []string{}, errors.New(fmt.Sprintf("Unclosed quote in command line: %s", cmd))
	}

	if current != "" {
		args = append(args, current)
	}

	if len(args) <= 0 {
		return "", []string{}, errors.New("Empty command line")
	}

	if len(args) == 1 {
		return args[0], []string{}, nil
	}

	return args[0], args[1:], nil
}
