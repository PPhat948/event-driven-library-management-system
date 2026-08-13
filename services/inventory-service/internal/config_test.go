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
		os.Unsetenv("SQS_QUEUE_URL")
		os.Unsetenv("SERVER_PORT")

		_, err := LoadConfig()
		if err == nil {
			t.Error("LoadConfig() expected error when required vars missing, got nil")
		}
	})

	t.Run("parses custom LOW_STOCK_THRESHOLD", func(t *testing.T) {
		os.Setenv("DATABASE_URL", "postgres://inv:inv@localhost:5433/inventory_db")
		os.Setenv("AWS_REGION", "us-east-1")
		os.Setenv("AWS_ENDPOINT_URL", "http://localhost:4566")
		os.Setenv("SNS_TOPIC_ARN", "arn:aws:sns:us-east-1:000:topic")
		os.Setenv("SQS_QUEUE_URL", "http://localhost:4566/000/queue")
		os.Setenv("SERVER_PORT", "8002")
		os.Setenv("LOW_STOCK_THRESHOLD", "5")

		cfg, err := LoadConfig()
		if err != nil {
			t.Fatalf("LoadConfig() unexpected error: %v", err)
		}

		if cfg.LowStockThreshold != 5 {
			t.Errorf("LowStockThreshold = %d, want 5", cfg.LowStockThreshold)
		}
	})

	t.Run("returns error on invalid LOW_STOCK_THRESHOLD format", func(t *testing.T) {
		os.Setenv("LOW_STOCK_THRESHOLD", "not-a-number")

		_, err := LoadConfig()
		if err == nil {
			t.Error("LoadConfig() expected error for non-numeric threshold, got nil")
		}

		os.Unsetenv("LOW_STOCK_THRESHOLD")
	})
}

func TestLoadDotEnv(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")

	content := "# Comment line\nINV_TEST_KEY=inv_val\n"
	if err := os.WriteFile(envPath, []byte(content), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	os.Unsetenv("INV_TEST_KEY")
	loadDotEnv(envPath)

	if got := os.Getenv("INV_TEST_KEY"); got != "inv_val" {
		t.Errorf("loadDotEnv() INV_TEST_KEY = %q, want %q", got, "inv_val")
	}
}
