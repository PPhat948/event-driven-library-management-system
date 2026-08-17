package events

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/rs/zerolog"
)

// Consumer polls an SQS queue in a long-poll loop and delegates each message
// to Handler. Messages are deleted only on success; on error they return to
// the queue for retry. After maxReceiveCount=3 retries (configured on the
// SQS queue itself), messages move to the DLQ.
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
		WaitTimeSeconds:     20, // long-poll — reduces empty receives
		VisibilityTimeout:   30, // 30s window for processing before retry
	})
	if err != nil {
		// ctx cancelled during long-poll — normal shutdown
		if ctx.Err() != nil {
			return
		}
		c.log.Error().Err(err).Msg("SQS ReceiveMessage error")
		return
	}

	for _, msg := range out.Messages {
		if err := c.handler.Handle(ctx, *msg.Body); err != nil {
			// Transient error — reduce visibility timeout to 1s so SQS retries immediately
			_, _ = c.sqsClient.ChangeMessageVisibility(ctx, &sqs.ChangeMessageVisibilityInput{
				QueueUrl:          aws.String(c.queueURL),
				ReceiptHandle:     msg.ReceiptHandle,
				VisibilityTimeout: 1,
			})
			c.log.Error().Err(err).
				Str("message_id", *msg.MessageId).
				Msg("handle failed — message will be retried by SQS in 1s")
			continue
		}

		// Success — delete from queue.
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
