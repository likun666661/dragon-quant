package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	DeepSeek DeepSeekConfig `yaml:"deepseek"`
}

type DeepSeekConfig struct {
	APIKey string `yaml:"api_key"`
}

func LoadConfig() (*Config, error) {
	f, err := os.Open("config.yaml")
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var cfg Config
	decoder := yaml.NewDecoder(f)
	err = decoder.Decode(&cfg)
	if err != nil {
		return nil, err
	}

	return &cfg, nil
}
