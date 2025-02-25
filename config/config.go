package config

import (
	"gopkg.in/yaml.v3"
	"os"
	"regexp"
)

type Action struct {
	Run []string `yaml:"run"`
}

type Trigger struct {
	Name           string `yaml:"name"`
	Regex          string `yaml:"regex"`
	RegexpCompiled *regexp.Regexp
	Actions        []Action `yaml:"actions"`
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
				return nil, err
			}
			config.Flows[i].Triggers[j].RegexpCompiled = re
		}
	}

	return &config, nil
}
