package events

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/rs/zerolog"
)

type Consumer struct {
	sqsClient *sqs.Client
	queueURL  string
	handler   *Handler
	log       zerolog.Logger
}

func NewConsumer(sqsClient *sqs.Client, queueURL string, handler *Handler, log zerolog.Logger) *Consumer {
	return &Consumer{
		sqsClient: sqsClient,
		queueURL:  queueURL,
		handler:   handler,
		log:       log,
	}
}

// Start blocks until ctx is cancelled. Run it in a goroutine.
func (c *Consumer) Start(ctx context.Context) {
	c.log.Info().Str("queue", c.queueURL).Msg("SQS consumer started")
	for {
		select {
		case <-ctx.Done():
			c.log.Info().Msg("SQS consumer stopped")
			return
		default:
			c.poll(ctx)
		}
	}
}

func (c *Consumer) poll(ctx context.Context) {
	out, err := c.sqsClient.ReceiveMessage(ctx, &sqs.ReceiveMessageInput{
		QueueUrl:            aws.String(c.queueURL),
		MaxNumberOfMessages: 10,
		WaitTimeSeconds:     20,
		VisibilityTimeout:   30,
	})
	if err != nil {
		if ctx.Err() != nil {
			return
		}
		c.log.Error().Err(err).Msg("SQS ReceiveMessage error")
		return
	}

	for _, msg := range out.Messages {
		if err := c.handler.Handle(ctx, *msg.Body); err != nil {
			c.log.Error().Err(err).
				Str("message_id", *msg.MessageId).
				Msg("handle failed — message will be retried by SQS")
			continue
		}

		if _, err := c.sqsClient.DeleteMessage(ctx, &sqs.DeleteMessageInput{
			QueueUrl:      aws.String(c.queueURL),
			ReceiptHandle: msg.ReceiptHandle,
		}); err != nil {
			c.log.Error().Err(err).
				Str("message_id", *msg.MessageId).
				Msg("delete message failed — may be processed twice (idempotency handles it)")
		}
	}
}
