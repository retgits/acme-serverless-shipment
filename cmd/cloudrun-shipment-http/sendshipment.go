package main

import (
	"bytes"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/getsentry/sentry-go"
	acmeserverless "github.com/retgits/acme-serverless"
	"github.com/retgits/acme-serverless-shipment/internal/shipper"
	"github.com/valyala/fasthttp"
)

// SendShipment ...
func SendShipment(ctx *fasthttp.RequestCtx) {
	// Unmarshal the ShipmentRequested event to a struct
	req, err := acmeserverless.UnmarshalShipmentRequested(ctx.Request.Body())
	if err != nil {
		if err != nil {
			ErrorHandler(ctx, "SendShipment", "UnmarshalShipmentRequested", err)
			return
		}
	}

	// Send a breadcrumb to Sentry with the shipment request
	sentry.AddBreadcrumb(&sentry.Breadcrumb{
		Category:  acmeserverless.ShipmentRequestedEventName,
		Timestamp: time.Now(),
		Level:     sentry.LevelInfo,
		Data:      acmeserverless.ToSentryMap(req.Data),
	})

	// Send the shipment data
	shipmentData := shipper.Sent(req.Data)

	// Send a breadcrumb to Sentry with the shipment status
	sentry.AddBreadcrumb(&sentry.Breadcrumb{
		Category:  acmeserverless.ShipmentSentEventName,
		Timestamp: time.Now(),
		Level:     sentry.LevelInfo,
		Data:      acmeserverless.ToSentryMap(shipmentData),
	})

	evt := acmeserverless.ShipmentSent{
		Metadata: acmeserverless.Metadata{
			Domain: acmeserverless.ShipmentDomain,
			Source: "SendShipment",
			Type:   acmeserverless.ShipmentSentEventName,
			Status: acmeserverless.DefaultSuccessStatus,
		},
		Data: shipmentData,
	}

	payload, err := evt.Marshal()
	if err != nil {
		ErrorHandler(ctx, "SendShipment", "Marshal", err)
		return
	}

	go handleDelivery(evt.Data)

	ctx.SetStatusCode(http.StatusOK)
	ctx.Write(payload)
}

func handleDelivery(shipmentData acmeserverless.ShipmentData) {
	// Wait for the delivery
	shipmentData = shipper.Delivered(shipmentData)

	// Create a new event with the new status of the shipment
	evt := acmeserverless.ShipmentSent{
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
		Timestamp: time.Now(),
		Level:     sentry.LevelInfo,
		Data:      acmeserverless.ToSentryMap(evt.Data),
	})

	b, _ := evt.Marshal()
	payload := bytes.NewReader(b)

	req, err := http.NewRequest("POST", os.Getenv("ORDER_URL"), payload)
	if err != nil {
		log.Printf("error building http request for order status: %s", err.Error())
		return
	}

	req.Header.Add("content-type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("error sending order status: %s", err.Error())
		return
	}
	log.Printf("order service accepted message with status: %s", res.Status)
}
