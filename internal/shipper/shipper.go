package shipper

import (
	"log"
	"math/rand"
	"time"

	"github.com/gofrs/uuid"
	acmeserverless "github.com/retgits/acme-serverless"
)

const (
	minDeliveryTime = 5
	maxDeliveryTime = 120
)

// Sent takes care of sending the shipment to the customer. This would be the interface between
// the ACME Serverless Fitness Shop and the shipper.
func Sent(r acmeserverless.ShipmentRequest) acmeserverless.ShipmentData {
	log.Printf("Hello, this is %s... We'll take care of your package!", r.Delivery)

	trackingnumber := uuid.Must(uuid.NewV4()).String()

	res := acmeserverless.ShipmentData{
		TrackingNumber: trackingnumber,
		OrderNumber:    r.OrderID,
		Status:         "shipped - pending delivery",
	}

	return res
}

// Delivered takes care of alerting the ACME Serverless Fitness Shop that the order has
// been delivered to the customer.
func Delivered(s acmeserverless.ShipmentData) acmeserverless.ShipmentData {
	d := deliveryTime(minDeliveryTime, maxDeliveryTime)
	log.Printf("Simulating delivery by sleeping for %d seconds", d)

	time.Sleep(time.Duration(d) * time.Second)

	s.Status = "delivered"

	return s
}

// deliveryTime generates a random number between the min and max values, to
// determine how long a go routine will sleep to simulate delivery. The min
// and max values are set during the initialization of the shipper.
func deliveryTime(min int, max int) int {
	rand.Seed(time.Now().UTC().UnixNano())
	return min + rand.Intn(max-min)
}
