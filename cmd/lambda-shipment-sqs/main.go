// Package main is a shipment service, because what is a shop without a way to ship your purchases?
//
// The Shipping service is part of the [ACME Fitness Serverless Shop](https://github.com/retgits/acme-serverless).
// The goal of this specific service is, as the name implies, to ship products using a wide variety of shipping
// suppliers.
package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/getsentry/sentry-go"
	acmeserverless "github.com/retgits/acme-serverless"
	"github.com/retgits/acme-serverless-shipment/internal/emitter/sqs"
	"github.com/retgits/acme-serverless-shipment/internal/shipper"
	wflambda "github.com/wavefronthq/wavefront-lambda-go"
)

// handler handles the SQS events and returns an error if anything goes wrong.
// The resulting event, if no error is thrown, is sent to an SQS queue.
func handler(request events.SQSEvent) error {
	// Initiialize a connection to Sentry to capture errors and traces
	sentry.Init(sentry.ClientOptions{
		Dsn: os.Getenv("SENTRY_DSN"),
		Transport: &sentry.HTTPSyncTransport{
			Timeout: time.Second * 3,
		},
		ServerName:  os.Getenv("FUNCTION_NAME"),
		Release:     os.Getenv("VERSION"),
		Environment: os.Getenv("STAGE"),
	})

	// Unmarshal the ShipmentRequested event to a struct
	req, err := acmeserverless.UnmarshalShipmentRequested([]byte(request.Records[0].Body))
	if err != nil {
		return handleError("unmarshaling shipment", err)
	}

	// Send a breadcrumb to Sentry with the shipment request
	sentry.AddBreadcrumb(&sentry.Breadcrumb{
		Category:  acmeserverless.ShipmentRequestedEventName,
		Timestamp: time.Now().Unix(),
		Level:     sentry.LevelInfo,
		Data:      acmeserverless.ToSentryMap(req.Data),
	})

	shipmentData := shipper.Sent(req.Data)

	evt := acmeserverless.ShipmentSent{
		Metadata: acmeserverless.Metadata{
			Domain: acmeserverless.ShipmentDomain,
			Source: "SendShipment",
			Type:   acmeserverless.ShipmentSentEventName,
			Status: acmeserverless.DefaultSuccessStatus,
		},
		Data: shipmentData,
	}

	// Send a breadcrumb to Sentry with the shipment status
	sentry.AddBreadcrumb(&sentry.Breadcrumb{
		Category:  acmeserverless.ShipmentSentEventName,
		Timestamp: time.Now().Unix(),
		Level:     sentry.LevelInfo,
		Data:      acmeserverless.ToSentryMap(evt.Data),
	})

	// Create a new SQS EventEmitter and send the event
	em := sqs.New()
	err = em.Send(evt)
	if err != nil {
		return handleError("sending event", err)
	}

	// Wait for the delivery
	shipmentData = shipper.Delivered(shipmentData)

	// Create a new event with the new status of the shipment
	evt = acmeserverless.ShipmentSent{
		Metadata: acmeserverless.Metadata{
			Domain: acmeserverless.ShipmentDomain,
			Source: "SendShipment",
			Type:   acmeserverless.ShipmentDeliveredEventName,
			Status: acmeserverless.DefaultSuccessStatus,
		},
		Data: shipmentData,
	}

	// Send a breadcrumb to Sentry with the shipment status
	sentry.AddBreadcrumb(&sentry.Breadcrumb{
		Category:  acmeserverless.ShipmentDeliveredEventName,
		Timestamp: time.Now().Unix(),
		Level:     sentry.LevelInfo,
		Data:      acmeserverless.ToSentryMap(evt.Data),
	})

	err = em.Send(evt)
	if err != nil {
		return handleError("sending event", err)
	}

	sentry.CaptureMessage(fmt.Sprintf("order %s successfully delivered", req.Data.OrderID))

	return nil
}

// handleError takes the activity where the error occured and the error object and sends a message to sentry.
// The original error is returned so it can be thrown.
func handleError(activity string, err error) error {
	log.Printf("error %s: %s", activity, err.Error())
	sentry.CaptureException(fmt.Errorf("error %s: %s", activity, err.Error()))
	return err
}

// The main method is executed by AWS Lambda and points to the handler
func main() {
	lambda.Start(wflambda.Wrapper(handler))
}
