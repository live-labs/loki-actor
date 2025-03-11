package actions

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/live-labs/lokiactor/config"
	"log/slog"
	"os/exec"
	"strings"
	"time"
)

type CMDAction struct {
	run []string
}

func NewCMDAction(cfg config.Action) *CMDAction {
	a := &CMDAction{
		run: make([]string, len(cfg.CmdRun)),
	}
	//  Copy the command to the action, so that we can't accidentally modify the original command
	copy(a.run, cfg.CmdRun)
	return a
}

func (a *CMDAction) Execute(ts time.Time, message string, labels map[string]string) error {

	// Replace the ${values.ts} and ${values.message} placeholders in the command with the actual values
	command := make([]string, len(a.run))
	copy(command, a.run)

	tsStr := ts.Format(RFC3339_MILLI)

	for i, v := range command {

		v = strings.ReplaceAll(v, "${values.ts}", tsStr)
		v = strings.ReplaceAll(v, "${values.message}", message)

		for lk, lv := range labels {
			v = strings.ReplaceAll(v, fmt.Sprintf("${labels.%s}", lk), lv)
		}

		command[i] = v
	}

	slog.Info("Running action", "action", strings.Join(command, " "))

	if len(command) == 0 {
		return errors.New("no command to run")
	}

	var cmd *exec.Cmd

	if len(command) == 1 {
		cmd = exec.Command(command[0])
	} else {
		cmd = exec.Command(command[0], command[1:]...)
	}

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	cmd.Stdout = stdout
	cmd.Stderr = stderr

	err := cmd.Run()

	for {
		line, err := stdout.ReadBytes('\n')
		if err != nil {
			break
		}
		slog.Info("Action", "stdout", string(line))
	}
	for {
		line, err := stderr.ReadBytes('\n')
		if err != nil {
			break
		}
		slog.Error("Action", "stderr", string(line))
	}

	if err != nil {
		return fmt.Errorf("failed to run command: %w", err)
	}

	return nil
}
