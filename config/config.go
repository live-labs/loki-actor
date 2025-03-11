package config

import (
	"bytes"
	"gopkg.in/yaml.v3"
	"os"
)

type Action struct {
	Type string `yaml:"type"` // slack, cmd

	// slack action
	SlackWebhookURL      string `yaml:"slack_webhook_url"`
	SlackTimeoutSec      int64  `yaml:"slack_timeout_sec"`
	SlackMessageTemplate string `yaml:"slack_message_template"`

	// cmd action
	CmdRun []string `yaml:"cmd_run"`
}

type Trigger struct {
	Name        string `yaml:"name"`
	Regex       string `yaml:"regex"`
	IgnoreRegex string `yaml:"ignore_regex"`

	Lines           int     `yaml:"lines"`
	Action          Action  `yaml:"action"`
	NextLinesAction *Action `yaml:"next_lines_action"` // if lines > 0
}

type Flow struct {
	Name     string    `yaml:"name"`
	Query    string    `yaml:"query"`
	Triggers []Trigger `yaml:"triggers"`
}

type Loki struct {
	Host string `yaml:"host"`
	Port int    `yaml:"port"`
}

type Config struct {
	Loki  Loki   `yaml:"loki"`
	Flows []Flow `yaml:"flows"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)

	var config Config
	if err := dec.Decode(&config); err != nil {
		return nil, err
	}

	return &config, nil
}
