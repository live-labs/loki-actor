package config

import (
	"bytes"
	"fmt"
	"gopkg.in/yaml.v3"
	"os"
)

type Action struct {
	Type string `yaml:"type"` // slack, cmd

	Extends string `yaml:"extends,omitempty"` // extends another action

	// slack action
	SlackWebhookURL      string `yaml:"slack_webhook_url,omitempty"`
	SlackTimeoutSec      int64  `yaml:"slack_timeout_sec,omitempty"`
	SlackMessageTemplate string `yaml:"slack_message_template,omitempty"`
	SlackConcat          int    `yaml:"slack_concat,omitempty"`
	SlackConctatPrefix   string `yaml:"slack_concat_prefix,omitempty"`
	SlackConcatSuffix    string `yaml:"slack_concat_suffix,omitempty"`

	// cmd action
	CmdRun []string `yaml:"cmd_run,omitempty"`
}

func (a Action) Derive(parent Action) Action {
	if a.SlackWebhookURL == "" && parent.SlackWebhookURL != "" {
		a.SlackWebhookURL = parent.SlackWebhookURL
	}
	if a.SlackTimeoutSec == 0 && parent.SlackTimeoutSec != 0 {
		a.SlackTimeoutSec = parent.SlackTimeoutSec
	}
	if a.SlackMessageTemplate == "" && parent.SlackMessageTemplate != "" {
		a.SlackMessageTemplate = parent.SlackMessageTemplate
	}
	if a.SlackConcat == 0 && parent.SlackConcat != 0 {
		a.SlackConcat = parent.SlackConcat
	}
	if a.SlackConctatPrefix == "" && parent.SlackConctatPrefix != "" {
		a.SlackConctatPrefix = parent.SlackConctatPrefix
	}
	if a.SlackConcatSuffix == "" && parent.SlackConcatSuffix != "" {
		a.SlackConcatSuffix = parent.SlackConcatSuffix
	}
	if len(a.CmdRun) == 0 && len(parent.CmdRun) > 0 {
		a.CmdRun = make([]string, len(parent.CmdRun))
		copy(a.CmdRun, parent.CmdRun)
	}
	if a.Type == "" && parent.Type != "" {
		a.Type = parent.Type
	}

	a.Extends = "" // clear the extends field to avoid circular references
	return a

}

type Trigger struct {
	Name        string `yaml:"name,omitempty"`
	Regex       string `yaml:"regex,omitempty"`
	IgnoreRegex string `yaml:"ignore_regex,omitempty"`

	Lines               int    `yaml:"lines,omitempty"`
	ActionName          string `yaml:"action,omitempty"`
	NextLinesActionName string `yaml:"next_lines_action,omitempty"` // if lines > 0

	Action          Action  `yaml:"loaded_action,omitempty"`
	NextLinesAction *Action `yaml:"loaded_next_lines_action,omitempty"` // if lines > 0
}

type Flow struct {
	Name     string    `yaml:"name,omitempty"`
	Query    string    `yaml:"query,omitempty"`
	Triggers []Trigger `yaml:"triggers,omitempty"`
}

type Loki struct {
	Host string `yaml:"host,omitempty"`
	Port int    `yaml:"port,omitempty"`
}

type Config struct {
	Loki    Loki              `yaml:"loki,omitempty"`
	Actions map[string]Action `yaml:"actions,omitempty"`
	Flows   []Flow            `yaml:"flows,omitempty"`
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

	// populate actions with fields from their base action

	done := false
	for !done {
		done = true
		for name, action := range config.Actions {
			if action.Extends != "" {
				done = false // at least one action is not done, so we need to scan again
				baseAction, ok := config.Actions[action.Extends]
				if !ok {
					return nil, fmt.Errorf("action %s extends unknown action %s", name, action.Extends)
				}
				if baseAction.Extends != "" {
					continue // first populate the base action
				}

				// Copy non-zero fields from the base action to the current action, if they are not set
				action = action.Derive(baseAction)
				config.Actions[name] = action
			}
		}
	}

	// populate triggers with their actions
	for i, flow := range config.Flows {
		for j, trigger := range flow.Triggers {
			action, ok := config.Actions[trigger.ActionName]
			if !ok {
				return nil, fmt.Errorf("trigger %s action %s not found", trigger.Name, trigger.ActionName)
			}
			trigger.Action = action
			config.Flows[i].Triggers[j] = trigger

			if trigger.Lines > 0 {
				nextAction, ok := config.Actions[trigger.NextLinesActionName]
				if !ok {
					return nil, fmt.Errorf("trigger %s next lines action %s not found", trigger.Name, trigger.NextLinesActionName)
				}
				trigger.NextLinesAction = &nextAction
				config.Flows[i].Triggers[j] = trigger
			}
		}
		config.Flows[i] = flow
	}

	return &config, nil
}
