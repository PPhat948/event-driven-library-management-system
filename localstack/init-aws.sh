#!/bin/bash
echo "Initializing LocalStack AWS SNS & SQS resources..."

export AWS_DEFAULT_REGION=us-east-1

# 1. Create Dead Letter Queue (DLQ)
awslocal sqs create-queue --queue-name library-dlq
DLQ_ARN=$(awslocal sqs get-queue-attributes --queue-url http://localhost:4566/000000000000/library-dlq --attribute-names QueueArn --query 'Attributes.QueueArn' --output text)

# Redrive policy for max 3 retries before DLQ
REDRIVE_POLICY="{\"deadLetterTargetArn\":\"$DLQ_ARN\",\"maxReceiveCount\":\"3\"}"

# 2. Create Service Queues with DLQ attached
awslocal sqs create-queue --queue-name inventory-book-events --attributes "{\"RedrivePolicy\":\"$REDRIVE_POLICY\"}"
awslocal sqs create-queue --queue-name notification-events --attributes "{\"RedrivePolicy\":\"$REDRIVE_POLICY\"}"

INV_QUEUE_ARN=$(awslocal sqs get-queue-attributes --queue-url http://localhost:4566/000000000000/inventory-book-events --attribute-names QueueArn --query 'Attributes.QueueArn' --output text)
NOTIF_QUEUE_ARN=$(awslocal sqs get-queue-attributes --queue-url http://localhost:4566/000000000000/notification-events --attribute-names QueueArn --query 'Attributes.QueueArn' --output text)

# 3. Create SNS Topic
awslocal sns create-topic --name library-events
TOPIC_ARN="arn:aws:sns:us-east-1:000000000000:library-events"

# 4. Subscribe Queues to SNS Topic
awslocal sns subscribe \
  --topic-arn "$TOPIC_ARN" \
  --protocol sqs \
  --notification-endpoint "$INV_QUEUE_ARN" \
  --attributes '{"RawMessageDelivery":"true"}'

awslocal sns subscribe \
  --topic-arn "$TOPIC_ARN" \
  --protocol sqs \
  --notification-endpoint "$NOTIF_QUEUE_ARN" \
  --attributes '{"RawMessageDelivery":"true"}'

echo "LocalStack AWS resources initialized successfully!"
awslocal sns list-topics
awslocal sqs list-queues
