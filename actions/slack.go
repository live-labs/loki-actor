package actions

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/live-labs/lokiactor/config"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

type SlackAction struct {
	webhookURL      string
	client          *http.Client
	messageTemplate string
}

func NewSlackAction(cfg config.Action) *SlackAction {
	return &SlackAction{
		webhookURL: cfg.SlackWebhookURL,
		client: &http.Client{
			Timeout: time.Duration(cfg.SlackTimeoutSec) * time.Second,
		},
		messageTemplate: cfg.SlackMessageTemplate,
	}
}

func (a *SlackAction) Execute(ts time.Time, message string, labels map[string]string) error {
	// instead of curl', '-X', 'POST', '-H', 'Content-Type: application/json', '-d', '{ "text": "${values.message}" }', 'https://hooks.slack.com/services/...'
	tsStr := ts.Format(RFC3339_MILLI)

	v := a.messageTemplate

	v = strings.ReplaceAll(v, "${values.ts}", tsStr)
	v = strings.ReplaceAll(v, "${values.message}", message)

	for lk, lv := range labels {
		v = strings.ReplaceAll(v, fmt.Sprintf("${labels.%s}", lk), lv)
	}

	payload := map[string]string{
		"text": v,
	}

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("error marshaling payload: %w", err)
	}

	req, err := http.NewRequest("POST", a.webhookURL, bytes.NewBuffer(jsonPayload))
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
