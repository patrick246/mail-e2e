package config_test

import (
	"os"
	"testing"

	"github.com/patrick246/mail-e2e/internal/config"
	"github.com/stretchr/testify/require"
)

func TestGet(t *testing.T) {
	tests := []struct {
		name   string
		config string
		env    map[string]string

		wantConfig config.Config
		wantError  error
	}{{
		name:       "empty yaml",
		config:     "",
		env:        nil,
		wantConfig: config.Config{},
	}, {
		name: "single target",
		config: `---
metricsPort: 8080
targets:
  - name: example.com
    smtp:
      hostname: smtp.example.com
      port: 25
      from: maile2e@example.com
      to: test@example.com
    imap:
      hostname: imap.example.com
      port: 993
      username: test@example.com
      password: somepassword
      insecureSkipVerify: false
`,
		wantConfig: config.Config{
			MetricsPort: 8080,
			Targets: []config.Target{{
				Name: "example.com",
				SMTP: config.SMTPHost{
					Hostname: "smtp.example.com",
					Port:     25,
					Username: "",
					Password: "",
					From:     "maile2e@example.com",
					To:       "test@example.com",
				},
				IMAP: config.IMAPHost{
					Hostname:           "imap.example.com",
					Port:               993,
					Username:           "test@example.com",
					Password:           "somepassword",
					InsecureSkipVerify: false,
				},
				Interval: 0,
			}},
		},
	}, {
		name: "single target with password via env",
		config: `---
metricsPort: 8080
targets:
  - name: example.com
    smtp:
      hostname: smtp.example.com
      port: 25
      from: maile2e@example.com
      to: test@example.com
    imap:
      hostname: imap.example.com
      port: 993
      username: test@example.com
      insecureSkipVerify: false
`,
		env: map[string]string{
			"MAILE2E_TARGET_EXAMPLE_COM_IMAP_PASSWORD": "somepassword",
			"MAILE2E_TARGET_EXAMPLE_COM_SMTP_PASSWORD": "somepassword",
		},
		wantConfig: config.Config{
			MetricsPort: 8080,
			Targets: []config.Target{{
				Name: "example.com",
				SMTP: config.SMTPHost{
					Hostname: "smtp.example.com",
					Port:     25,
					Username: "",
					Password: "somepassword",
					From:     "maile2e@example.com",
					To:       "test@example.com",
				},
				IMAP: config.IMAPHost{
					Hostname:           "imap.example.com",
					Port:               993,
					Username:           "test@example.com",
					Password:           "somepassword",
					InsecureSkipVerify: false,
				},
				Interval: 0,
			}},
		},
	}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for k, v := range tt.env {
				t.Setenv(k, v)
			}

			f, err := os.CreateTemp(t.TempDir(), "*.yml")
			require.NoError(t, err)

			_, err = f.WriteString(tt.config)
			require.NoError(t, err)

			t.Setenv("MAILE2E_CONFIG_FILE", f.Name())

			cfg, err := config.Get()
			if tt.wantError != nil {
				require.ErrorIs(t, err, tt.wantError)
			}

			require.Equal(t, tt.wantConfig, cfg)
		})
	}
}
