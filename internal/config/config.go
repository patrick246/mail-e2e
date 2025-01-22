package config

import (
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	MetricsPort uint16   `yaml:"metricsPort"`
	Targets     []Target `yaml:"targets"`
}

type Target struct {
	Name     string        `yaml:"name"`
	SMTP     SMTPHost      `yaml:"smtp"`
	IMAP     IMAPHost      `yaml:"imap"`
	Interval time.Duration `yaml:"interval"`
}

type SMTPHost struct {
	Hostname string `yaml:"hostname"`
	Port     uint16 `yaml:"port"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
	From     string `yaml:"from"`
	To       string `yaml:"to"`
}

type IMAPHost struct {
	Hostname           string `yaml:"hostname"`
	Port               uint16 `yaml:"port"`
	Username           string `yaml:"username"`
	Password           string `yaml:"password"`
	InsecureSkipVerify bool   `yaml:"insecureSkipVerify"`
}

func Get() (Config, error) {
	configFileLocation := os.Getenv("MAILE2E_CONFIG_FILE")
	if configFileLocation == "" {
		configFileLocation = "/etc/mail-e2e/config.yaml"
	}

	//nolint:mnd // default values
	var cfg Config

	configFile, err := os.Open(configFileLocation)
	if err != nil {
		return Config{}, nil
	}

	err = yaml.NewDecoder(configFile).Decode(&cfg)
	if err != nil {
		return Config{}, err
	}

	return cfg, nil
}
