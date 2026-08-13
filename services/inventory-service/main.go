package main

import (
	"context"
	"embed"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/rs/zerolog"

	"inventory-service/internal"
	"inventory-service/internal/events"
	"inventory-service/internal/handler"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

func main() {
	log := zerolog.New(os.Stdout).With().Timestamp().Logger()

	cfg, err := internal.LoadConfig()
	if err != nil {
		log.Fatal().Err(err).Msg("load config")
	}

	// Graceful shutdown: cancel propagates to both consumer goroutine and HTTP server.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := internal.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatal().Err(err).Msg("connect to postgres")
	}
	defer pool.Close()

	if err := internal.RunMigrations(migrationFiles, cfg.DatabaseURL); err != nil {
		log.Fatal().Err(err).Msg("run migrations")
	}
	log.Info().Msg("migrations OK")

	awsCfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion(cfg.AWSRegion),
		awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider("test", "test", ""),
		),
	)
	if err != nil {
		log.Fatal().Err(err).Msg("load aws config")
	}

	snsClient := sns.NewFromConfig(awsCfg, func(o *sns.Options) {
		o.BaseEndpoint = aws.String(cfg.AWSEndpoint)
	})
	sqsClient := sqs.NewFromConfig(awsCfg, func(o *sqs.Options) {
		o.BaseEndpoint = aws.String(cfg.AWSEndpoint)
	})

	pub := events.NewPublisher(snsClient, cfg.SNSTopicARN)
	repo := internal.NewInventoryRepo(pool)
	evtHandler := events.NewHandler(pool, repo, pub, cfg.LowStockThreshold, log)
	consumer := events.NewConsumer(sqsClient, cfg.SQSQueueURL, evtHandler, log)

	// Run SQS consumer in a background goroutine.
	// It exits when ctx is cancelled (signal received).
	go consumer.Start(ctx)

	r := chi.NewRouter()
	r.Use(handler.CorrelationID)
	r.Use(handler.RequestLogger(log))
	r.Use(chimw.Recoverer)

	h := handler.New(repo, log)
	r.Mount("/", h.Routes())

	srv := &http.Server{
		Addr:    ":" + cfg.ServerPort,
		Handler: r,
	}

	// Shutdown HTTP server when ctx is cancelled.
	go func() {
		<-ctx.Done()
		log.Info().Msg("shutting down HTTP server")
		srv.Shutdown(context.Background())
	}()

	log.Info().
		Str("port", cfg.ServerPort).
		Int("low_stock_threshold", cfg.LowStockThreshold).
		Msg("starting inventory-service")

	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal().Err(err).Msg("server stopped unexpectedly")
	}
}
