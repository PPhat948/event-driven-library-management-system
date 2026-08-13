package events

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sns"
)

type Publisher struct {
	client   *sns.Client
	topicARN string
}

func NewPublisher(client *sns.Client, topicARN string) *Publisher {
	return &Publisher{client: client, topicARN: topicARN}
}

func (p *Publisher) Publish(ctx context.Context, eventType, correlationID string, payload any) error {
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	env := newEnvelope(eventType, correlationID, payloadBytes)

	body, err := json.Marshal(env)
	if err != nil {
		return fmt.Errorf("marshal envelope: %w", err)
	}

	_, err = p.client.Publish(ctx, &sns.PublishInput{
		TopicArn: aws.String(p.topicARN),
		Message:  aws.String(string(body)),
	})
	return err
}
