package sqsadapter

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"github.com/rafaelgscc/desafio-backend-go-junglegaming/internal/platform/config"
)

var ErrTransportRequired = errors.New("SQS transport is required")

type Client interface {
	GetQueueUrl(context.Context, *sqs.GetQueueUrlInput, ...func(*sqs.Options)) (*sqs.GetQueueUrlOutput, error)
	GetQueueAttributes(context.Context, *sqs.GetQueueAttributesInput, ...func(*sqs.Options)) (*sqs.GetQueueAttributesOutput, error)
	ReceiveMessage(context.Context, *sqs.ReceiveMessageInput, ...func(*sqs.Options)) (*sqs.ReceiveMessageOutput, error)
	DeleteMessage(context.Context, *sqs.DeleteMessageInput, ...func(*sqs.Options)) (*sqs.DeleteMessageOutput, error)
	ChangeMessageVisibility(context.Context, *sqs.ChangeMessageVisibilityInput, ...func(*sqs.Options)) (*sqs.ChangeMessageVisibilityOutput, error)
	SendMessage(context.Context, *sqs.SendMessageInput, ...func(*sqs.Options)) (*sqs.SendMessageOutput, error)
}

type Transport struct {
	client        Client
	queueURL      string
	eventQueueURL string
}

func NewTransport(sqsConfig config.SQSConfig) (*Transport, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	loadOptions := []func(*awsconfig.LoadOptions) error{awsconfig.WithRegion(sqsConfig.Region)}
	if sqsConfig.Endpoint != "" {
		loadOptions = append(loadOptions, awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider("test", "test", ""),
		))
	}
	awsConfig, err := awsconfig.LoadDefaultConfig(ctx, loadOptions...)
	if err != nil {
		return nil, fmt.Errorf("load AWS config: %w", err)
	}
	client := sqs.NewFromConfig(awsConfig, func(options *sqs.Options) {
		if sqsConfig.Endpoint != "" {
			options.BaseEndpoint = aws.String(sqsConfig.Endpoint)
		}
	})
	return newTransport(ctx, client, sqsConfig.QueueName, sqsConfig.EventQueueName)
}

func newTransport(
	ctx context.Context,
	client Client,
	queueName string,
	eventQueueName string,
) (*Transport, error) {
	if client == nil {
		return nil, ErrTransportRequired
	}
	output, err := client.GetQueueUrl(ctx, &sqs.GetQueueUrlInput{QueueName: aws.String(queueName)})
	if err != nil {
		return nil, fmt.Errorf("resolve SQS queue %q: %w", queueName, err)
	}
	if output.QueueUrl == nil || *output.QueueUrl == "" {
		return nil, fmt.Errorf("resolve SQS queue %q: empty URL", queueName)
	}
	eventOutput, err := client.GetQueueUrl(ctx, &sqs.GetQueueUrlInput{
		QueueName: aws.String(eventQueueName),
	})
	if err != nil {
		return nil, fmt.Errorf("resolve SQS queue %q: %w", eventQueueName, err)
	}
	if eventOutput.QueueUrl == nil || *eventOutput.QueueUrl == "" {
		return nil, fmt.Errorf("resolve SQS queue %q: empty URL", eventQueueName)
	}
	return &Transport{
		client: client, queueURL: *output.QueueUrl, eventQueueURL: *eventOutput.QueueUrl,
	}, nil
}

func (transport *Transport) Ping(ctx context.Context) error {
	for _, queueURL := range []string{transport.queueURL, transport.eventQueueURL} {
		_, err := transport.client.GetQueueAttributes(ctx, &sqs.GetQueueAttributesInput{
			QueueUrl:       aws.String(queueURL),
			AttributeNames: []types.QueueAttributeName{types.QueueAttributeNameQueueArn},
		})
		if err != nil {
			return fmt.Errorf("ping SQS queue %q: %w", queueURL, err)
		}
	}
	return nil
}
