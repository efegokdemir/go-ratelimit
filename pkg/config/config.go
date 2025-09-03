package config

import (
	"os"

	"gopkg.in/yaml.v2"
)

type Config struct {
	Logging             LoggingConfig             `yaml:"logging"`
}

type LoggingConfig struct {
	ConsoleLevel string `yaml:"console_level"`
	FileLevel    string `yaml:"file_level"`
	File         string `yaml:"file"`
}

var config Config

// LoadConfig reads the configuration from a YAML file
func LoadConfig(configFile string) (Config, error) {

	data, err := os.ReadFile(configFile)

	if err != nil {
		return config, err
	}

	err = yaml.Unmarshal(data, &config)
	if err != nil {
		return config, err
	}

	return config, nil
}
