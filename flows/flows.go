package flows

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/coder/websocket"
	"github.com/live-labs/lokiactor/config"
	"github.com/live-labs/lokiactor/loki"
	"log/slog"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

const RFC3339_MILLI = "2006-01-02T15:04:05.000Z"

type Flow struct {
	ctx     context.Context
	cfg     config.Flow
	lokiCfg config.Loki

	continuationAction *config.Action // the action to run for the multiline flow
	continuationLines  int
}

func New(ctx context.Context, cfg config.Flow, lokiCfg config.Loki) *Flow {
	return &Flow{
		ctx:     ctx,
		cfg:     cfg,
		lokiCfg: lokiCfg,
	}
}

func (f *Flow) Run() {

	slog.Info("Starting flow", "name", f.cfg.Name)

	delay := time.Duration(0)

	for {
		select {
		case <-f.ctx.Done():
			slog.Info("Flow cancelled", "name", f.cfg.Name)
		case <-time.After(delay):
		}

		start := time.Now().Add(-delay) // start from now - delay (in case of retry)
		//start := time.Now().Add(-time.Minute * 30)
		lokiURL := url.URL{
			Scheme: "ws",
			Host:   fmt.Sprintf("%s:%d", f.lokiCfg.Host, f.lokiCfg.Port),
			Path:   "/loki/api/v1/tail",
		}
		query := lokiURL.Query()
		query.Set("query", f.cfg.Query)
		query.Set("start", strconv.FormatInt(start.UnixNano(), 10))
		lokiURL.RawQuery = query.Encode()

		urlStr := lokiURL.String()

		// after initial 0 delay, retry every retryInterval
		delay = 5 * time.Second

		slog.Info("Connecting to Loki stream", "url", urlStr)
		conn, response, err := websocket.Dial(f.ctx, urlStr, nil)
		if err != nil {
			slog.Error("Failed to connect to Loki stream", "error", err)
			continue
		}

		if response.StatusCode != 101 {
			fmt.Println("Unexpected status code", response.StatusCode)
			continue
		}

		slog.Info("Connected to Loki stream", "url", urlStr)

		conn.SetReadLimit(-1)

		f.processMessages(conn)
	}
}

func (f *Flow) processMessages(conn *websocket.Conn) {
	for {
		websocketMessageType, websocketMessage, err := conn.Read(f.ctx)

		if err != nil {
			slog.Error("Failed to read from websocket", "error", err)
			break
		}

		if websocketMessageType != websocket.MessageText {
			slog.Warn("Unexpected websocket message type", "type", websocketMessageType)
			continue
		}

		var event loki.Event

		err = json.Unmarshal(websocketMessage, &event)
		if err != nil {
			slog.Error("Failed to unmarshal websocket message", "error", err)
			continue
		}

		// we have an event, so now we have to go through all of the triggers
		// and see if any of them match the event

		f.processLokiEvent(event)

	}
}

func (f *Flow) processLokiEvent(event loki.Event) {
	for _, stream := range event.Streams {
		lines := stream.Values
		for _, line := range lines {
			f.processLogLine(line, stream.Details)
		}
	}
}

func (f *Flow) processLogLine(line []string, labels map[string]string) {
	// available as ${values.ts}
	ts := line[0]

	tsInt, err := strconv.ParseInt(ts, 10, 64)
	if err != nil {
		slog.Error("Failed to parse timestamp", "error", err)
		return
	}

	timestamp := time.Unix(0, tsInt)

	// available as ${values.message}
	message := line[1]

	// available as ${values.message_escaped}
	messageEscaped := strings.ReplaceAll(message, "\"", "\\\"")
	messageEscaped = "\"" + messageEscaped + "\""

	if f.continuationLines <= 0 && f.continuationAction != nil {
		f.continuationAction = nil
		slog.Info("Finished multiline action")
	}

	if f.continuationAction != nil {
		slog.Debug("Continuing multiline action", "message", message)
		f.runAction(*f.continuationAction, timestamp, message, messageEscaped, labels)
		f.continuationLines--
		return
	}

	for _, trigger := range f.cfg.Triggers {
		if !trigger.RegexpCompiled.MatchString(message) {
			continue
		}

		slog.Debug("Trigger matched", "trigger", trigger.Name, "regexp", trigger.Regex, "message", message)

		if trigger.IgnoreRegexpCompiled != nil && trigger.IgnoreRegexpCompiled.MatchString(message) {
			slog.Debug("Trigger ignored", "trigger", trigger.Name, "ignore_regexp", trigger.IgnoreRegex, "message", message)
			continue
		}

		for _, action := range trigger.Actions {
			f.runAction(action, timestamp, message, messageEscaped, labels)

		}

		if trigger.ContinuationLines > 0 {
			if len(trigger.Actions) > 0 {
				f.continuationAction = &trigger.Actions[0] // default
			}

			if trigger.ContinuationAction != nil {
				f.continuationAction = trigger.ContinuationAction
			}

			if f.continuationAction == nil {
				slog.Warn("No continuation action defined")
				f.continuationLines = 0
				break
			}

			f.continuationLines = trigger.ContinuationLines

		}

		break // only one trigger per event
	}
}

func (f *Flow) runAction(action config.Action, timestamp time.Time, message string, messageEscaped string, labels map[string]string) {
	slog.Info("Preparing action", "action", action.Run)

	// substitude ${values.ts|message} and ${labels.*} in action.Run
	command := make([]string, len(action.Run))
	copy(command, action.Run)

	timeStr := timestamp.Format(RFC3339_MILLI)

	for i, v := range command {

		v = strings.ReplaceAll(v, "${values.ts}", timeStr)
		v = strings.ReplaceAll(v, "${values.message}", message)
		v = strings.ReplaceAll(v, "${values.message_escaped}", messageEscaped)

		for lk, lv := range labels {
			v = strings.ReplaceAll(v, fmt.Sprintf("${labels.%s}", lk), lv)
		}

		command[i] = v
	}

	slog.Info("Running action", "action", strings.Join(command, " "))

	if len(command) == 0 {
		slog.Warn("No command to run")
		return
	}

	var cmd *exec.Cmd

	if len(command) == 1 {
		cmd = exec.Command(command[0])
	} else {
		cmd = exec.Command(command[0], command[1:]...)
	}

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Run()
	if err != nil {
		slog.Error("Failed to run action", "error", err)
	} else {
		slog.Info("Action completed successfully")
	}
}
