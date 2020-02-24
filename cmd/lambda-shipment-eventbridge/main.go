package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/getsentry/sentry-go"
	shipment "github.com/retgits/acme-serverless-shipment"
	"github.com/retgits/acme-serverless-shipment/internal/emitter"
	"github.com/retgits/acme-serverless-shipment/internal/emitter/eventbridge"
	"github.com/retgits/acme-serverless-shipment/internal/shipper"
)

func handler(request json.RawMessage) error {
	sentrySyncTransport := sentry.NewHTTPSyncTransport()
	sentrySyncTransport.Timeout = time.Second * 3

	sentry.Init(sentry.ClientOptions{
		Dsn:         os.Getenv("SENTRY_DSN"),
		Transport:   sentrySyncTransport,
		ServerName:  os.Getenv("FUNCTION_NAME"),
		Release:     os.Getenv("VERSION"),
		Environment: os.Getenv("STAGE"),
	})

	req, err := shipment.UnmarshalShipmentEvent(request)
	if err != nil {
		sentry.CaptureException(fmt.Errorf("error unmarshalling shipment event: %s", err.Error()))
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

	sentry.AddBreadcrumb(&sentry.Breadcrumb{
		Category:  "SentShipment",
		Timestamp: time.Now().Unix(),
		Level:     sentry.LevelInfo,
		Data: map[string]interface{}{
			"TrackingNumber": data.TrackingNumber,
			"OrderNumber":    data.OrderNumber,
			"Status":         data.Status,
		},
	})

	em := eventbridge.New()
	err = em.Send(evt)
	if err != nil {
		sentry.CaptureException(fmt.Errorf("error sending shipment event: %s", err.Error()))
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

	sentry.AddBreadcrumb(&sentry.Breadcrumb{
		Category:  "DeliveredShipment",
		Timestamp: time.Now().Unix(),
		Level:     sentry.LevelInfo,
		Data: map[string]interface{}{
			"TrackingNumber": data.TrackingNumber,
			"OrderNumber":    data.OrderNumber,
			"Status":         data.Status,
		},
	})

	err = em.Send(evt)
	if err != nil {
		sentry.CaptureException(fmt.Errorf("error sending DeliveredShipment event: %s", err.Error()))
		return err
	}

	sentry.CaptureMessage(fmt.Sprintf("order [%s] successfully delivered", req.Request.OrderID))

	return nil
}

// The main method is executed by AWS Lambda and points to the handler
func main() {
	lambda.Start(handler)
}
