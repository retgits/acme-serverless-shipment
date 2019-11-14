package main

import (
	"log"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sqs"
	"github.com/kelseyhightower/envconfig"
	"github.com/retgits/shipment/shipper"

	wflambda "github.com/wavefronthq/wavefront-lambda-go"
)

var wfAgent = wflambda.NewWavefrontAgent(&wflambda.WavefrontConfig{})

// config is the struct that is used to keep track of all environment variables
type config struct {
	AWSRegion     string `required:"true" split_words:"true" envconfig:"REGION"`
	ResponseQueue string `required:"true" split_words:"true" envconfig:"RESPONSE_QUEUE"`
}

var c config

func handler(request events.SQSEvent) error {
	// Get configuration set using environment variables
	err := envconfig.Process("", &c)
	if err != nil {
		log.Printf("error starting function: %s", err.Error())
		return err
	}

	// Create an AWS session
	awsSession := session.Must(session.NewSession(&aws.Config{
		Region: aws.String(c.AWSRegion),
	}))

	// Create a SQS session
	sqsService := sqs.New(awsSession)

	for _, record := range request.Records {
		shipment, err := shipper.Send(record.Body)
		if err != nil {
			log.Printf("error sending shipment: %s", err.Error())
			break
		}

		s, err := shipment.Marshal()
		if err != nil {
			log.Printf("error marshalling shipment: %s", err.Error())
			break
		}

		sendMessageInput := &sqs.SendMessageInput{
			QueueUrl:    aws.String(c.ResponseQueue),
			MessageBody: aws.String(s),
		}

		_, err = sqsService.SendMessage(sendMessageInput)
		if err != nil {
			log.Printf("error sending shipment message: %s", err.Error())
			break
		}

		s, err = shipper.Delivered(shipment)
		if err != nil {
			log.Printf("error delivering shipment: %s", err.Error())
			break
		}

		sendMessageInput = &sqs.SendMessageInput{
			QueueUrl:    aws.String(c.ResponseQueue),
			MessageBody: aws.String(s),
		}

		_, err = sqsService.SendMessage(sendMessageInput)
		if err != nil {
			log.Printf("error sending delivery message: %s", err.Error())
			break
		}
	}

	return nil
}

// The main method is executed by AWS Lambda and points to the handler
func main() {
	lambda.Start(wfAgent.WrapHandler(handler))
}
