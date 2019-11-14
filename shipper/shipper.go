package shipper

import (
	"log"
	"math/rand"
	"time"

	"github.com/gofrs/uuid"
)

const (
	minDeliveryTime = 5
	maxDeliveryTime = 120
)

func Send(message string) (*Shipment, error) {
	msg, err := unmarshalOrder([]byte(message))
	if err != nil {
		log.Printf("error unmarshaling request: %s", err.Error())
		return nil, err
	}

	log.Printf("Hello, this is %s... We'll take care of your package!", msg.Delivery)

	trackingnumber := uuid.Must(uuid.NewV4()).String()

	res := &Shipment{
		TrackingNumber: trackingnumber,
		OrderNumber:    msg.OrderID,
		Status:         "shipped - pending delivery",
	}

	return res, err
}

func Delivered(s *Shipment) (string, error) {
	d := deliveryTime(minDeliveryTime, maxDeliveryTime)
	log.Printf("Simulating delivery by sleeping for %d seconds", d)

	time.Sleep(time.Duration(d) * time.Second)

	s.Status = "delivered"

	str, err := s.Marshal()
	if err != nil {
		log.Printf("error marshalling shipment: %s", err.Error())
		return "", err
	}

	return str, nil
}

// DeliveryTime generates a random number between the min and max values, to
// determine how long a go routine will sleep to simulate delivery. The min
// and max values are set during the initialization of the shipper.
func deliveryTime(min int, max int) int {
	rand.Seed(time.Now().UTC().UnixNano())
	return min + rand.Intn(max-min)
}
