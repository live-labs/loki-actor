package config

import (
	"fmt"
	"gopkg.in/yaml.v3"
	"os"
	"regexp"
)

type Action struct {
	Run []string `yaml:"run"`
}

type Trigger struct {
	Name                 string  `yaml:"name"`
	Regex                string  `yaml:"regex"`
	IgnoreRegex          string  `yaml:"ignore_regex"`
	DurationMs           int     `yaml:"duration_ms"` // duration in milliseconds to extract from the start of the message
	ContinuationAction   *Action `yaml:"continuation_action"`
	RegexpCompiled       *regexp.Regexp
	IgnoreRegexpCompiled *regexp.Regexp
	Actions              []Action `yaml:"actions"`
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

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	for i, flow := range config.Flows {
		for j, trigger := range flow.Triggers {
			re, err := regexp.Compile(trigger.Regex)
			if err != nil {
				return nil, fmt.Errorf("failed to compile trigger %s regex %q: %w", trigger.Name, trigger.Regex, err)
			}
			config.Flows[i].Triggers[j].RegexpCompiled = re

			if trigger.IgnoreRegex != "" {
				ignoreRe, err := regexp.Compile(trigger.IgnoreRegex)
				if err != nil {
					return nil, fmt.Errorf("failed to compile trigger %s ignore_regex %q: %w", trigger.Name, trigger.IgnoreRegex, err)
				}
				config.Flows[i].Triggers[j].IgnoreRegexpCompiled = ignoreRe
			}

		}
	}

	return &config, nil
}
