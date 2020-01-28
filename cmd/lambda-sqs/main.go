package main

import (
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/retgits/acme-serverless-shipment"
	"github.com/retgits/acme-serverless-shipment/internal/emitter"
	"github.com/retgits/acme-serverless-shipment/internal/emitter/sqs"
	"github.com/retgits/acme-serverless-shipment/internal/shipper"
)

func handler(request events.SQSEvent) error {
	req, err := shipment.UnmarshalShipmentEvent([]byte(request.Records[0].Body))
	if err != nil {
		return err
	}

	data := shipper.Sent(req.Request)

	evt := emitter.Event{
		Metadata: shipment.Metadata{
			Domain: "Shipment",
			Source: "SendShipment",
			Type:   "SentShipment",
			Status: "success",
		},
		Data: data,
	}

	em := sqs.New()
	err = em.Send(evt)
	if err != nil {
		return err
	}

	data = shipper.Delivered(data)

	evt = emitter.Event{
		Metadata: shipment.Metadata{
			Domain: "Shipment",
			Source: "SendShipment",
			Type:   "DeliveredShipment",
			Status: "success",
		},
		Data: data,
	}

	return em.Send(evt)
}

// The main method is executed by AWS Lambda and points to the handler
func main() {
	lambda.Start(handler)
}
