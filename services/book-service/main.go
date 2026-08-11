package main

import (
	"context"
	"embed"
	"net/http"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/rs/zerolog"

	"book-service/internal"
	"book-service/internal/events"
	"book-service/internal/handler"
)

// embed works here because main.go is at the same level as migrations/
//
//go:embed migrations/*.sql
var migrationFiles embed.FS

func main() {
	log := zerolog.New(os.Stdout).With().Timestamp().Logger()

	cfg, err := internal.LoadConfig()
	if err != nil {
		log.Fatal().Err(err).Msg("load config")
	}

	ctx := context.Background()

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

	pub := events.NewPublisher(snsClient, cfg.SNSTopicARN)
	books := internal.NewBookRepo(pool)
	borrows := internal.NewBorrowRepo(pool)

	r := chi.NewRouter()
	r.Use(handler.CorrelationID)
	r.Use(handler.RequestLogger(log))
	r.Use(chimw.Recoverer)

	h := handler.New(pool, books, borrows, pub, log)
	r.Mount("/", h.Routes())

	log.Info().Str("port", cfg.ServerPort).Msg("starting book-service")
	if err := http.ListenAndServe(":"+cfg.ServerPort, r); err != nil {
		log.Fatal().Err(err).Msg("server stopped")
	}
}
