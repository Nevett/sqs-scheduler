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

	messages := make(chan *ReceivedMessage)
	go messageHandler.getMessages(messages)
	for {
		message := <-messages
		go messageHandler.handleMessage(message)
	}
}

type MessageHandler struct {
	queueUrl   *string
	sqsService *sqs.SQS
}

type ReceivedMessage struct {
	payload          *webhook.Message
	sqsReceiptHandle *string
}

func (handler MessageHandler) getMessages(messages chan<- *ReceivedMessage) {
	maxMessages := int64(10)
	waitTimeSeconds := int64(20)

	for {
		receiveResult, err := handler.sqsService.ReceiveMessage(&sqs.ReceiveMessageInput{
			MaxNumberOfMessages: &maxMessages,
			QueueUrl:            handler.queueUrl,
			WaitTimeSeconds:     &waitTimeSeconds,
		})

		if err != nil {
			log.Print(err)
		}

		fmt.Printf("Received %d messages\n", len(receiveResult.Messages))
		for i := 0; i < len(receiveResult.Messages); i++ {
			var msg webhook.Message
			err := json.Unmarshal([]byte(*receiveResult.Messages[i].Body), &msg)
			if err != nil {
				log.Println(err)
				go handler.deleteSqsMessage(receiveResult.Messages[i].ReceiptHandle)
			} else {
				messages <- &ReceivedMessage{
					sqsReceiptHandle: receiveResult.Messages[i].ReceiptHandle,
					payload:          &msg,
				}
			}
		}
	}
}

func (handler MessageHandler) handleMessage(message *ReceivedMessage) {

	delay := message.payload.ScheduledDelivery.Sub(time.Now().UTC())

	deliveryTimer := time.NewTimer(delay)
	sqsInterval := time.NewTicker(time.Duration(20) * time.Second)
	deliverySuccess := make(chan int)

done:
	for {
		select {
		case <-deliveryTimer.C:
			go handler.deliverMessage(message, deliverySuccess)
		case <-sqsInterval.C:
			go handler.delaySqs(message.sqsReceiptHandle)
		case <-deliverySuccess:
			break done
		}
	}

	handler.deleteSqsMessage(message.sqsReceiptHandle)
}

func (handler MessageHandler) deleteSqsMessage(receiptHandle *string) {
	handler.sqsService.DeleteMessage(&sqs.DeleteMessageInput{
		QueueUrl:      handler.queueUrl,
		ReceiptHandle: receiptHandle,
	})
}

func (handler MessageHandler) deliverMessage(message *ReceivedMessage, success chan int) {
	fmt.Printf("Delivering message %s\n", message.payload.MessageId)
	success <- 1
}

func (handler MessageHandler) delaySqs(receiptHandle *string) {
	delay := int64(30)
	handler.sqsService.ChangeMessageVisibility(&sqs.ChangeMessageVisibilityInput{
		QueueUrl:          handler.queueUrl,
		ReceiptHandle:     receiptHandle,
		VisibilityTimeout: &delay,
	})
}
