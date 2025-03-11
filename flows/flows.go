package flows

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/coder/websocket"
	"github.com/live-labs/lokiactor/actions"
	"github.com/live-labs/lokiactor/config"
	"github.com/live-labs/lokiactor/loki"
	"github.com/live-labs/lokiactor/triggers"
	"log/slog"
	"net/url"
	"strconv"
	"time"
)

type Flow struct {
	ctx      context.Context
	name     string
	query    string
	triggers []*triggers.Trigger

	lokiCfg config.Loki

	continuationAction actions.Action // the action to run for the multiline flow
	continuationLines  int
}

func New(ctx context.Context, cfg config.Flow, lokiCfg config.Loki) (*Flow, error) {

	tgz := make([]*triggers.Trigger, len(cfg.Triggers))

	for i, trigger := range cfg.Triggers {
		t, err := triggers.New(ctx, trigger)
		if err != nil {
			return nil, fmt.Errorf("failed to create trigger %s: %w", trigger.Name, err)
		}

		tgz[i] = t
	}

	return &Flow{
		ctx:      ctx,
		name:     cfg.Name,
		query:    cfg.Query,
		triggers: tgz,

		lokiCfg: lokiCfg,
	}, nil
}

func (f *Flow) Name() string {
	return f.name
}

func (f *Flow) Run() {

	slog.Info("Starting flow", "name", f.name)

	delay := time.Duration(0)

	for {
		select {
		case <-f.ctx.Done():
			slog.Info("Flow cancelled", "name", f.name)
		case <-time.After(delay):
		}

		start := time.Now().Add(-delay) // start from now - delay (in case of retry)
		lokiURL := url.URL{
			Scheme: "ws",
			Host:   fmt.Sprintf("%s:%d", f.lokiCfg.Host, f.lokiCfg.Port),
			Path:   "/loki/api/v1/tail",
		}
		query := lokiURL.Query()
		query.Set("query", f.query)
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

	if f.continuationLines <= 0 && f.continuationAction != nil {
		f.continuationAction = nil
		slog.Info("Finished multiline action")
	}

	if f.continuationAction != nil {
		slog.Debug("Continuing multiline action", "message", message)

		err = f.continuationAction.Execute(timestamp, message, labels)
		f.continuationLines--

		if err != nil {
			slog.Error("Failed to run continuation action", "error", err)
		}

		return
	}

	for _, trigger := range f.triggers {
		if !trigger.Regex.MatchString(message) {
			continue
		}

		slog.Debug("Trigger matched", "trigger", trigger.Name, "regexp", trigger.Regex.String(), "message", message)

		if trigger.IgnoreRegex != nil && trigger.IgnoreRegex.MatchString(message) {
			slog.Debug("Trigger ignored", "trigger", trigger.Name, "ignore_regexp", trigger.IgnoreRegex.String(), "message", message)
			continue
		}

		err := trigger.Action.Execute(timestamp, message, labels)
		if err != nil {
			slog.Error("Failed to run action", "error", err)
			return
		}

		if trigger.Lines > 0 {

			slog.Debug("Starting multiline action", "trigger", trigger.Name, "lines", trigger.Lines)

			f.continuationLines = trigger.Lines
			f.continuationAction = trigger.NextLinesAction

			err = f.continuationAction.Execute(timestamp, message, labels)
			if err != nil {
				slog.Error("Failed to run continuation action", "error", err)
			}

			return

		}

		break // only one trigger per event
	}
}
