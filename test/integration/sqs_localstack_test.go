package integration_test

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
)

func TestLocalStackProvisionsFIFOQueueAndDLQ(t *testing.T) {
	endpoint := os.Getenv("TEST_SQS_ENDPOINT")
	if endpoint == "" {
		t.Skip("TEST_SQS_ENDPOINT is not configured")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	awsConfig, err := awsconfig.LoadDefaultConfig(
		ctx,
		awsconfig.WithRegion("us-east-1"),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider("test", "test", "")),
	)
	if err != nil {
		t.Fatal(err)
	}
	client := sqs.NewFromConfig(awsConfig, func(options *sqs.Options) {
		options.BaseEndpoint = aws.String(endpoint)
	})
	mainQueue, err := client.GetQueueUrl(ctx, &sqs.GetQueueUrlInput{
		QueueName: aws.String("wager-transactions.fifo"),
	})
	if err != nil {
		t.Fatalf("resolve main queue: %v", err)
	}
	if _, err := client.GetQueueUrl(ctx, &sqs.GetQueueUrlInput{
		QueueName: aws.String("wager-transactions-dlq.fifo"),
	}); err != nil {
		t.Fatalf("resolve DLQ: %v", err)
	}
	eventQueue, err := client.GetQueueUrl(ctx, &sqs.GetQueueUrlInput{
		QueueName: aws.String("integration-events.fifo"),
	})
	if err != nil {
		t.Fatalf("resolve integration event queue: %v", err)
	}
	attributes, err := client.GetQueueAttributes(ctx, &sqs.GetQueueAttributesInput{
		QueueUrl: mainQueue.QueueUrl,
		AttributeNames: []types.QueueAttributeName{
			types.QueueAttributeNameFifoQueue,
			types.QueueAttributeNameRedrivePolicy,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	var redrivePolicy struct {
		DeadLetterTargetARN string `json:"deadLetterTargetArn"`
		MaxReceiveCount     string `json:"maxReceiveCount"`
	}
	if err := json.Unmarshal(
		[]byte(attributes.Attributes[string(types.QueueAttributeNameRedrivePolicy)]),
		&redrivePolicy,
	); err != nil {
		t.Fatalf("decode redrive policy: %v", err)
	}
	if attributes.Attributes[string(types.QueueAttributeNameFifoQueue)] != "true" ||
		redrivePolicy.DeadLetterTargetARN == "" || redrivePolicy.MaxReceiveCount != "5" {
		t.Fatalf("queue attributes = %#v", attributes.Attributes)
	}

	body := `{"messageId":"localstack-test","type":"WagerTransactionRequested"}`
	if _, err := client.SendMessage(ctx, &sqs.SendMessageInput{
		QueueUrl: mainQueue.QueueUrl, MessageBody: aws.String(body),
		MessageGroupId: aws.String("wallet-test"), MessageDeduplicationId: aws.String("localstack-test"),
	}); err != nil {
		t.Fatalf("send message: %v", err)
	}
	received, err := client.ReceiveMessage(ctx, &sqs.ReceiveMessageInput{
		QueueUrl: mainQueue.QueueUrl, MaxNumberOfMessages: 1, WaitTimeSeconds: 2,
	})
	if err != nil || len(received.Messages) != 1 || aws.ToString(received.Messages[0].Body) != body {
		t.Fatalf("received = %#v, error = %v", received.Messages, err)
	}
	_, _ = client.DeleteMessage(ctx, &sqs.DeleteMessageInput{
		QueueUrl: mainQueue.QueueUrl, ReceiptHandle: received.Messages[0].ReceiptHandle,
	})
	if _, err := client.SendMessage(ctx, &sqs.SendMessageInput{
		QueueUrl: eventQueue.QueueUrl, MessageBody: aws.String(`{"eventId":"event-1"}`),
		MessageGroupId: aws.String("wallet-1"), MessageDeduplicationId: aws.String("event-1"),
	}); err != nil {
		t.Fatalf("send integration event: %v", err)
	}
}
