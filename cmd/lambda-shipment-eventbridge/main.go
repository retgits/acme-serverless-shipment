// Package main is a shipment service, because what is a shop without a way to ship your purchases?
//
// The Shipping service is part of the [ACME Fitness Serverless Shop](https://github.com/retgits/acme-serverless).
// The goal of this specific service is, as the name implies, to ship products using a wide variety of shipping
// suppliers.
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/getsentry/sentry-go"
	shipment "github.com/retgits/acme-serverless-shipment"
	"github.com/retgits/acme-serverless-shipment/internal/emitter/eventbridge"
	"github.com/retgits/acme-serverless-shipment/internal/shipper"
)

// handler handles the EventBridge events and returns an error if anything goes wrong.
// The resulting event, if no error is thrown, is sent to an EventBridge bus.
func handler(request json.RawMessage) error {
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
	req, err := shipment.UnmarshalShipmentRequested(request)
	if err != nil {
		return handleError("unmarshaling shipment", err)
	}

	// Send a breadcrumb to Sentry with the shipment request
	sentry.AddBreadcrumb(&sentry.Breadcrumb{
		Category:  shipment.ShipmentRequestedEvent,
		Timestamp: time.Now().Unix(),
		Level:     sentry.LevelInfo,
		Data:      req.Data.ToMap(),
	})

	shipmentData := shipper.Sent(req.Data)

	evt := shipment.ShipmentSent{
		Metadata: shipment.Metadata{
			Domain: shipment.Domain,
			Source: "SendShipment",
			Type:   shipment.ShipmentSentEvent,
			Status: "success",
		},
		Data: shipmentData,
	}

	// Send a breadcrumb to Sentry with the shipment status
	sentry.AddBreadcrumb(&sentry.Breadcrumb{
		Category:  shipment.ShipmentSentEvent,
		Timestamp: time.Now().Unix(),
		Level:     sentry.LevelInfo,
		Data:      req.Data.ToMap(),
	})

	// Create a new EventBridgee EventEmitter and send the event
	em := eventbridge.New()
	err = em.Send(evt)
	if err != nil {
		return handleError("sending event", err)
	}

	// Wait for the delivery
	shipmentData = shipper.Delivered(shipmentData)

	// Create a new event with the new status of the shipment
	evt = shipment.ShipmentSent{
		Metadata: shipment.Metadata{
			Domain: shipment.Domain,
			Source: "SendShipment",
			Type:   shipment.ShipmentDeliveredEvent,
			Status: "success",
		},
		Data: shipmentData,
	}

	// Send a breadcrumb to Sentry with the shipment status
	sentry.AddBreadcrumb(&sentry.Breadcrumb{
		Category:  shipment.ShipmentDeliveredEvent,
		Timestamp: time.Now().Unix(),
		Level:     sentry.LevelInfo,
		Data:      req.Data.ToMap(),
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
	lambda.Start(handler)
}
