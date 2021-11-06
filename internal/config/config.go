package config

import (
	"github.com/brumhard/alligotor"
	"os"
)

type Config struct {
	MetricsPort uint16
	Targets     []Target
}

type Target struct {
	Name string
	SMTP SMTPHost
	IMAP IMAPHost
}

type SMTPHost struct {
	Hostname string
	Port     uint16
	Username string
	Password string
	From     string
	To       string
}

type IMAPHost struct {
	Hostname           string
	Port               uint16
	Username           string
	Password           string
	InsecureSkipVerify bool
}

func Get() (Config, error) {
	configFileLocation := os.Getenv("MAILE2E_CONFIG_FILE")
	if configFileLocation == "" {
		configFileLocation = "/etc/mail-e2e/config.yaml"
	}
	cfgSource := alligotor.New(
		alligotor.NewEnvSource("MAILE2E"),
		alligotor.NewFlagsSource(),
		alligotor.NewFilesSource(configFileLocation),
	)

	cfg := Config{
		MetricsPort: 8080,
	}
	err := cfgSource.Get(&cfg)
	if err != nil {
		return Config{}, err
	}

	return cfg, nil
}
