package internal

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

type Config struct {
	DatabaseURL string
	AWSRegion   string
	AWSEndpoint string
	SQSQueueURL string
	ServerPort  string
	LogLevel    string
}

func LoadConfig() (Config, error) {
	loadDotEnv(".env")

	cfg := Config{
		DatabaseURL: os.Getenv("DATABASE_URL"),
		AWSRegion:   os.Getenv("AWS_REGION"),
		AWSEndpoint: os.Getenv("AWS_ENDPOINT_URL"),
		SQSQueueURL: os.Getenv("SQS_QUEUE_URL"),
		ServerPort:  os.Getenv("SERVER_PORT"),
		LogLevel:    os.Getenv("LOG_LEVEL"),
	}

	required := map[string]string{
		"DATABASE_URL":     cfg.DatabaseURL,
		"AWS_REGION":       cfg.AWSRegion,
		"AWS_ENDPOINT_URL": cfg.AWSEndpoint,
		"SQS_QUEUE_URL":    cfg.SQSQueueURL,
		"SERVER_PORT":      cfg.ServerPort,
	}
	for key, val := range required {
		if val == "" {
			return Config{}, fmt.Errorf("missing required env var: %s", key)
		}
	}

	return cfg, nil
}

func loadDotEnv(filepath string) {
	file, err := os.Open(filepath)
	if err != nil {
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			val := strings.TrimSpace(parts[1])
			if os.Getenv(key) == "" {
				os.Setenv(key, val)
			}
		}
	}
}
