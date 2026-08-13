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
		os.Unsetenv("SQS_QUEUE_URL")
		os.Unsetenv("SERVER_PORT")

		_, err := LoadConfig()
		if err == nil {
			t.Error("LoadConfig() expected error when required vars missing, got nil")
		}
	})

	t.Run("returns Config when all required env vars present", func(t *testing.T) {
		os.Setenv("DATABASE_URL", "postgres://notif:notif@localhost:5434/notification_db")
		os.Setenv("AWS_REGION", "us-east-1")
		os.Setenv("AWS_ENDPOINT_URL", "http://localhost:4566")
		os.Setenv("SQS_QUEUE_URL", "http://localhost:4566/000/queue")
		os.Setenv("SERVER_PORT", "8003")
		os.Setenv("LOG_LEVEL", "info")

		cfg, err := LoadConfig()
		if err != nil {
			t.Fatalf("LoadConfig() unexpected error: %v", err)
		}

		if cfg.ServerPort != "8003" {
			t.Errorf("ServerPort = %q, want %q", cfg.ServerPort, "8003")
		}
	})
}

func TestLoadDotEnv(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")

	content := "# Comment line\nNOTIF_TEST_KEY=notif_val\n"
	if err := os.WriteFile(envPath, []byte(content), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	os.Unsetenv("NOTIF_TEST_KEY")
	loadDotEnv(envPath)

	if got := os.Getenv("NOTIF_TEST_KEY"); got != "notif_val" {
		t.Errorf("loadDotEnv() NOTIF_TEST_KEY = %q, want %q", got, "notif_val")
	}
}
