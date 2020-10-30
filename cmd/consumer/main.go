package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	_ "net/http/pprof"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sqs"
	"go.nevett.me/sqsscheduler/lib/webhook"
)

func main() {
	go func() {
		fmt.Println("Starting pprof")
		fmt.Println(http.ListenAndServe(":6061", nil))
	}()
	queueName := "test-queue"
	endpoint := "http://localstack:4566"
	region := "us-east-1"
	awsSession := session.Must(session.NewSessionWithOptions(session.Options{
		Config: aws.Config{
			Endpoint: &endpoint,
			Region:   &region,
			Credentials: credentials.NewCredentials(
				&credentials.StaticProvider{
					Value: credentials.Value{AccessKeyID: "localstack", SecretAccessKey: "localstack"}})},
	}))

	sqsService := sqs.New(awsSession)

	queueUrl, err := sqsService.GetQueueUrl(&sqs.GetQueueUrlInput{QueueName: &queueName})
	if err != nil {
		panic(err)
	}

	messageHandler := MessageHandler{
		queueUrl:   queueUrl.QueueUrl,
		sqsService: sqsService,
	}

	for {
		maxMessages := int64(10)
		waitTimeSeconds := int64(20)
		messages, err := sqsService.ReceiveMessage(&sqs.ReceiveMessageInput{
			MaxNumberOfMessages: &maxMessages,
			QueueUrl:            queueUrl.QueueUrl,
			WaitTimeSeconds:     &waitTimeSeconds,
		})

		if err != nil {
			log.Print(err)
		}

		fmt.Printf("Received %d messages\n", len(messages.Messages))
		for i := 0; i < len(messages.Messages); i++ {
			go messageHandler.handleMessage(messages.Messages[i])
		}
	}
}

type MessageHandler struct {
	queueUrl   *string
	sqsService *sqs.SQS
}

func (handler MessageHandler) handleMessage(message *sqs.Message) {
	var msg webhook.Message
	err := json.Unmarshal([]byte(*message.Body), &msg)
	if err != nil {
		panic(err)
	}

	delay := msg.ScheduledDelivery.Sub(time.Now().UTC())

	deliveryTimer := time.NewTimer(delay)
	sqsInterval := time.NewTicker(time.Duration(20) * time.Second)
	deliverySuccess := make(chan int)

	for {
		select {
		case <-deliveryTimer.C:
			go handler.deliverMessage(&msg, deliverySuccess)
		case <-sqsInterval.C:
			go handler.delaySqs(message)
		case <-deliverySuccess:
			break
		}
	}

	handler.sqsService.DeleteMessage(&sqs.DeleteMessageInput{
		QueueUrl:      handler.queueUrl,
		ReceiptHandle: message.ReceiptHandle,
	})
}

func (handler MessageHandler) deliverMessage(webhook *webhook.Message, success chan int) {
	fmt.Printf("Delivering message %s\n", webhook.MessageId)
	success <- 1
}

func (handler MessageHandler) delaySqs(message *sqs.Message) {
	delay := int64(30)
	handler.sqsService.ChangeMessageVisibility(&sqs.ChangeMessageVisibilityInput{
		QueueUrl:          handler.queueUrl,
		ReceiptHandle:     message.ReceiptHandle,
		VisibilityTimeout: &delay,
	})
}
