package shipper

import (
	"log"
	"math/rand"
	"time"

	"github.com/gofrs/uuid"
	"github.com/retgits/shipment"
	"github.com/retgits/shipment/internal/emitter"
)

const (
	minDeliveryTime = 5
	maxDeliveryTime = 120
)

// Sent ...
func Sent(r shipment.Request) emitter.Data {
	log.Printf("Hello, this is %s... We'll take care of your package!", r.Delivery)

	trackingnumber := uuid.Must(uuid.NewV4()).String()

	res := emitter.Data{
		TrackingNumber: trackingnumber,
		OrderNumber:    r.OrderID,
		Status:         "shipped - pending delivery",
	}

	return res
}

// Delivered ...
func Delivered(s emitter.Data) emitter.Data {
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
