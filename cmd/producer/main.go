package main

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sqs"
	"go.nevett.me/sqsscheduler/lib/webhook"

	"net/http"
	_ "net/http/pprof"
)

func main() {
	go func() {
		fmt.Println(http.ListenAndServe(":6060", nil))
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

	fmt.Println(queueUrl.QueueUrl)

	i := 0
	for {
		var delivery time.Time
		if rand.Intn(2) == 0 {
			// send at random point in next 60s
			delivery = time.Now().UTC().Add(time.Duration(rand.Int31n(60000)) * time.Millisecond)
		} else {
			// send at top of the next minute
			delivery = time.Now().Add(time.Minute).Truncate(time.Minute)
		}
		msg := &webhook.Message{
			MessageId:         fmt.Sprintf("message-%d", i),
			ScheduledDelivery: delivery,
		}

		jsonBytes, err := json.Marshal(msg)
		if err != nil {
			panic(err)
		}
		jsonStr := string(jsonBytes)

		sqsService.SendMessage(&sqs.SendMessageInput{
			QueueUrl:    queueUrl.QueueUrl,
			MessageBody: &jsonStr,
		})

		time.Sleep(100 * time.Millisecond)
		i++
	}
}
