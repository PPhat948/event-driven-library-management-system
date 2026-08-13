package internal

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	DatabaseURL       string
	AWSRegion         string
	AWSEndpoint       string
	SNSTopicARN       string
	SQSQueueURL       string
	ServerPort        string
	LogLevel          string
	LowStockThreshold int
}

func LoadConfig() (Config, error) {
	loadDotEnv(".env")

	threshold := 2
	if v := os.Getenv("LOW_STOCK_THRESHOLD"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return Config{}, fmt.Errorf("invalid LOW_STOCK_THRESHOLD: %w", err)
		}
		threshold = n
	}

	cfg := Config{
		DatabaseURL:       os.Getenv("DATABASE_URL"),
		AWSRegion:         os.Getenv("AWS_REGION"),
		AWSEndpoint:       os.Getenv("AWS_ENDPOINT_URL"),
		SNSTopicARN:       os.Getenv("SNS_TOPIC_ARN"),
		SQSQueueURL:       os.Getenv("SQS_QUEUE_URL"),
		ServerPort:        os.Getenv("SERVER_PORT"),
		LogLevel:          os.Getenv("LOG_LEVEL"),
		LowStockThreshold: threshold,
	}

	required := map[string]string{
		"DATABASE_URL":     cfg.DatabaseURL,
		"AWS_REGION":       cfg.AWSRegion,
		"AWS_ENDPOINT_URL": cfg.AWSEndpoint,
		"SNS_TOPIC_ARN":    cfg.SNSTopicARN,
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
