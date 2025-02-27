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

var multilineUntil time.Time       // the time until the multiline state is valid
var multilineAction *config.Action // the action to run for the multiline flow

func Run(ctx context.Context, flow config.Flow, loki config.Loki) {

	slog.Info("Starting flow", "name", flow.Name)

	delay := time.Duration(0)

	for {
		select {
		case <-ctx.Done():
			slog.Info("Flow cancelled", "name", flow.Name)
		case <-time.After(delay):
		}

		start := time.Now().Add(-delay) // start from now - delay (in case of retry)
		lokiURL := url.URL{
			Scheme: "ws",
			Host:   fmt.Sprintf("%s:%d", loki.Host, loki.Port),
			Path:   "/loki/api/v1/tail",
		}
		query := lokiURL.Query()
		query.Set("query", flow.Query)
		query.Set("start", strconv.FormatInt(start.UnixNano(), 10))
		lokiURL.RawQuery = query.Encode()

		urlStr := lokiURL.String()

		// after initial 0 delay, retry every retryInterval
		delay = 5 * time.Second

		slog.Info("Connecting to Loki stream", "url", urlStr)
		conn, response, err := websocket.Dial(ctx, urlStr, nil)
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

		processMessages(ctx, flow, conn)
	}
}

func processMessages(ctx context.Context, flow config.Flow, conn *websocket.Conn) {
	for {
		websocketMessageType, websocketMessage, err := conn.Read(ctx)

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

		processLokiEvent(event, flow)

	}
}

func processLokiEvent(event loki.Event, flow config.Flow) {
	for _, stream := range event.Streams {
		lines := stream.Values
		for _, line := range lines {
			processLogLine(line, stream.Details, flow)
		}
	}
}

func processLogLine(line []string, labels map[string]string, flow config.Flow) {
	// available as ${values.ts}
	ts := line[0]
	// available as ${values.message}
	message := line[1]

	// available as ${values.message_escaped}
	messageEscaped := strings.ReplaceAll(message, "\"", "\\\"")
	messageEscaped = "\"" + messageEscaped + "\""

	if multilineAction != nil && multilineUntil.After(time.Now()) {
		multilineAction = nil
		slog.Info("Multiline action finished")
	}

	if multilineAction != nil {
		runAction(*multilineAction, ts, message, messageEscaped, labels)
		return
	}

	for _, trigger := range flow.Triggers {
		if !trigger.RegexpCompiled.MatchString(message) {
			continue
		}

		slog.Debug("Trigger matched", "trigger", trigger.Name, "regexp", trigger.Regex, "message", message)

		if trigger.IgnoreRegexpCompiled != nil && trigger.IgnoreRegexpCompiled.MatchString(message) {
			slog.Debug("Trigger ignored", "trigger", trigger.Name, "ignore_regexp", trigger.IgnoreRegex, "message", message)
			continue
		}

		for _, action := range trigger.Actions {
			runAction(action, ts, message, messageEscaped, labels)

		}

		if trigger.DurationMs > 0 {
			multilineAction = &trigger.Actions[0] // default

			if trigger.ContinuationAction != nil {
				multilineAction = trigger.ContinuationAction
			}

			multilineUntil = time.Now().Add(time.Duration(trigger.DurationMs) * time.Millisecond)
		}

		break // only one trigger per event
	}
}

func runAction(action config.Action, ts string, message string, messageEscaped string, labels map[string]string) {
	slog.Info("Preparing action", "action", action.Run)

	// substitude ${values.ts|message} and ${labels.*} in action.Run
	command := make([]string, len(action.Run))
	copy(command, action.Run)

	for i, v := range command {

		v = strings.ReplaceAll(v, "${values.ts}", ts)
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
		slog.Error("Failed to run command", "error", err)
	} else {
		slog.Info("Command completed successfully")
	}
}
