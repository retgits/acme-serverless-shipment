package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/getsentry/sentry-go"
	sentryhttp "github.com/getsentry/sentry-go/http"
	acmeserverless "github.com/retgits/acme-serverless"
	"github.com/retgits/acme-serverless-shipment/internal/shipper"
	gcrwavefront "github.com/retgits/gcr-wavefront"
)

func handler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	// Set CORS headers for the preflight request
	case http.MethodOptions:
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization")
		w.Header().Set("Access-Control-Allow-Methods", "POST")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Max-Age", "3600")
		w.WriteHeader(http.StatusNoContent)
		return
	// Handle the Shipment request
	case http.MethodPost:
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		bytes, err := ioutil.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "400 - Bad request", http.StatusBadRequest)
		}
		res, err := handleShipment(bytes)
		if err != nil {
			http.Error(w, fmt.Sprintf("400 - Bad request: %s", err.Error()), http.StatusBadRequest)
		}
		payload, _ := res.Marshal()
		w.WriteHeader(http.StatusOK)
		w.Write(payload)
		go handleDelivery(res.Data)
	// Disallow all other HTTP methods
	default:
		http.Error(w, "405 - Method Not Allowed", http.StatusMethodNotAllowed)
	}
}

func handleShipment(bytes []byte) (acmeserverless.ShipmentSent, error) {
	// Unmarshal the ShipmentRequested event to a struct
	req, err := acmeserverless.UnmarshalShipmentRequested(bytes)
	if err != nil {
		return acmeserverless.ShipmentSent{}, err
	}

	// Send a breadcrumb to Sentry with the shipment request
	sentry.AddBreadcrumb(&sentry.Breadcrumb{
		Category:  acmeserverless.ShipmentRequestedEventName,
		Timestamp: time.Now().Unix(),
		Level:     sentry.LevelInfo,
		Data:      acmeserverless.ToSentryMap(req.Data),
	})

	// Send the shipment data
	shipmentData := shipper.Sent(req.Data)

	// Send a breadcrumb to Sentry with the shipment status
	sentry.AddBreadcrumb(&sentry.Breadcrumb{
		Category:  acmeserverless.ShipmentSentEventName,
		Timestamp: time.Now().Unix(),
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

	return evt, nil
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
		Timestamp: time.Now().Unix(),
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

func main() {
	// Initiialize a connection to Sentry to capture errors and traces
	if err := sentry.Init(sentry.ClientOptions{
		Dsn: os.Getenv("SENTRY_DSN"),
		Transport: &sentry.HTTPSyncTransport{
			Timeout: time.Second * 3,
		},
		ServerName:  os.Getenv("K_SERVICE"),
		Release:     os.Getenv("VERSION"),
		Environment: os.Getenv("STAGE"),
	}); err != nil {
		log.Fatalf("error configuring sentry: %s", err.Error())
	}

	// Create an instance of sentryhttp
	sentryHandler := sentryhttp.New(sentryhttp.Options{})

	// Set configuration parameters
	wfconfig := &gcrwavefront.WavefrontConfig{
		Server:        os.Getenv("WAVEFRONT_URL"),
		Token:         os.Getenv("WAVEFRONT_TOKEN"),
		BatchSize:     10000,
		MaxBufferSize: 50000,
		FlushInterval: 1,
		Source:        "acmeserverless",
		MetricPrefix:  "acmeserverless.gcr.shipment",
		PointTags:     make(map[string]string),
	}

	// Initialize the Wavefront sender
	if err := wfconfig.ConfigureSender(); err != nil {
		log.Fatalf("error configuring wavefront: %s", err.Error())
	}

	// Wrap the sentryHandler with the Wavefront middleware to make sure all events
	// are sent to sentry before sending data to Wavefront
	http.HandleFunc("/", wfconfig.WrapHandlerFunc(sentryHandler.HandleFunc(handler)))

	// Get the port
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// Start the server
	log.Printf("start shipment server on port %s", port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%s", port), nil))
}
