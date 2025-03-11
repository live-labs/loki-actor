package actions

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/live-labs/lokiactor/config"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

const tick = 5 * time.Second

type SlackAction struct {
	webhookURL      string
	client          *http.Client
	messageTemplate string
	c               chan []byte
}

func NewSlackAction(ctx context.Context, cfg config.Action) *SlackAction {
	sa := &SlackAction{
		webhookURL: cfg.SlackWebhookURL,
		client: &http.Client{
			Timeout: time.Duration(cfg.SlackTimeoutSec) * time.Second,
		},
		messageTemplate: cfg.SlackMessageTemplate,
	}

	if cfg.SlackConcat <= 0 {
		return sa
	}

	sa.c = make(chan []byte, 10)

	go func() {

		t := time.NewTicker(tick)

		var buffer *bytes.Buffer
		var n int

		mkBuffer := func() {
			buffer = &bytes.Buffer{}
			buffer.WriteString(cfg.SlackConctatPrefix)
			n = 0
		}

		appendBuffer := func(msg []byte) {
			if n == 0 {
				t.Reset(tick) // reset the timer on first message to avoid immediate send
			}
			if buffer == nil {
				mkBuffer() // create a new buffer if it doesn't exist
			}
			buffer.Write(msg)
			buffer.WriteRune('\n')
			n++
		}

		sendBuffer := func() error {
			buffer.WriteString(cfg.SlackConcatSuffix)

			payload := map[string]string{
				"text": buffer.String(),
			}

			jsonPayload, err := json.Marshal(payload)
			if err != nil {
				return fmt.Errorf("error marshaling payload: %w", err)
			}

			err = sa.send(bytes.NewReader(jsonPayload))
			mkBuffer()
			if err != nil {
				return err
			}
			return nil
		}

		for {
			select {
			case <-ctx.Done():
				if n > 0 {
					// Final attempt to send remaining messages
					err := sendBuffer()
					if err != nil {
						slog.Error("Error sending final messages to Slack", "error", err)
					}
				}
				t.Stop()
				return

			case msg := <-sa.c:
				appendBuffer(msg)

				if n >= cfg.SlackConcat {
					err := sendBuffer()
					if err != nil {
						slog.Error("Error sending message to Slack", "error", err)
					}
					t.Reset(time.Second * 5) // reset the timer
				}

			case <-t.C:
				if n > 0 {
					err := sendBuffer()
					if err != nil {
						slog.Error("Error sending message to Slack", "error", err)
					}
				}
			}
		}
	}()

	return sa
}

func (a *SlackAction) send(r io.Reader) error {

	req, err := http.NewRequest("POST", a.webhookURL, r)
	if err != nil {
		return fmt.Errorf("error creating HTTP request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := a.client.Do(req)
	if err != nil {
		return fmt.Errorf("error sending HTTP request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	slog.Debug("Message successfully sent to Slack")
	return nil
}

func (a *SlackAction) Execute(ts time.Time, message string, labels map[string]string) error {
	tsStr := ts.Format(RFC3339_MILLI)

	v := a.messageTemplate

	v = strings.ReplaceAll(v, "${values.ts}", tsStr)
	v = strings.ReplaceAll(v, "${values.message}", message)

	for lk, lv := range labels {
		v = strings.ReplaceAll(v, fmt.Sprintf("${labels.%s}", lk), lv)
	}

	if a.c == nil {
		// send the message immediately

		payload := map[string]string{
			"text": v,
		}

		jsonPayload, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("error marshaling payload: %w", err)
		}

		return a.send(bytes.NewReader(jsonPayload))
	}

	select {
	case a.c <- []byte(v):
	case <-time.After(time.Millisecond * 200):
		return fmt.Errorf("slack action channel is full, message dropped")
	}
	return nil

}
