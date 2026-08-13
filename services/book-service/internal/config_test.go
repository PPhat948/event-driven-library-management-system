package internal

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	t.Run("returns error when required env vars missing", func(t *testing.T) {
		os.Unsetenv("DATABASE_URL")
		os.Unsetenv("AWS_REGION")
		os.Unsetenv("AWS_ENDPOINT_URL")
		os.Unsetenv("SNS_TOPIC_ARN")
		os.Unsetenv("SERVER_PORT")

		_, err := LoadConfig()
		if err == nil {
			t.Error("LoadConfig() expected error when env vars missing, got nil")
		}
	})

	t.Run("returns Config when all required env vars present", func(t *testing.T) {
		os.Setenv("DATABASE_URL", "postgres://user:pass@localhost:5432/db")
		os.Setenv("AWS_REGION", "us-east-1")
		os.Setenv("AWS_ENDPOINT_URL", "http://localhost:4566")
		os.Setenv("SNS_TOPIC_ARN", "arn:aws:sns:us-east-1:000:topic")
		os.Setenv("SERVER_PORT", "8001")
		os.Setenv("LOG_LEVEL", "debug")

		cfg, err := LoadConfig()
		if err != nil {
			t.Fatalf("LoadConfig() unexpected error: %v", err)
		}

		if cfg.DatabaseURL != "postgres://user:pass@localhost:5432/db" {
			t.Errorf("DatabaseURL = %q, want %q", cfg.DatabaseURL, "postgres://user:pass@localhost:5432/db")
		}
		if cfg.ServerPort != "8001" {
			t.Errorf("ServerPort = %q, want %q", cfg.ServerPort, "8001")
		}
		if cfg.LogLevel != "debug" {
			t.Errorf("LogLevel = %q, want %q", cfg.LogLevel, "debug")
		}
	})
}

func TestLoadDotEnv(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")

	content := "# Comment line\n\nTEST_CONFIG_KEY=test_val\n"
	if err := os.WriteFile(envPath, []byte(content), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	os.Unsetenv("TEST_CONFIG_KEY")
	loadDotEnv(envPath)

	if got := os.Getenv("TEST_CONFIG_KEY"); got != "test_val" {
		t.Errorf("loadDotEnv() TEST_CONFIG_KEY = %q, want %q", got, "test_val")
	}
}
