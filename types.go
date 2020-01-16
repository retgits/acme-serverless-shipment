package shipment

import (
	"encoding/json"
)

// Metadata ...
type Metadata struct {
	// Domain represents the the event came from (like Payment or Order)
	Domain string `json:"domain"`
	// Source represents the function the event came from (like ValidateCreditCard or SubmitOrder)
	Source string `json:"source"`
	// Type respresents the type of event this is (like CreditCardValidated)
	Type string `json:"type"`
	// Status represents the current status of the event (like Success)
	Status string `json:"status"`
}

// Request ...
type Request struct {
	OrderID  string `json:"_id"`
	Delivery string `json:"delivery"`
}

// Event ...
type Event struct {
	Metadata Metadata `json:"metadata"`
	Request  Request  `json:"data"`
}

// UnmarshalShipmentEvent parses the JSON-encoded data and stores the result in an Event
func UnmarshalShipmentEvent(data []byte) (Event, error) {
	var r Event
	err := json.Unmarshal(data, &r)
	return r, err
}
