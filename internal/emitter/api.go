package emitter

import "encoding/json"

import "github.com/retgits/shipment"

// Data ...
type Data struct {
	TrackingNumber string `json:"trackingNumber"`
	OrderNumber    string `json:"orderNumber"`
	Status         string `json:"status"`
}

// Marshal returns the JSON encoding of Data
func (r *Data) Marshal() (string, error) {
	s, err := json.Marshal(r)
	if err != nil {
		return "", err
	}
	return string(s), nil
}

// Event ...
type Event struct {
	Metadata shipment.Metadata `json:"metadata"`
	Data     Data              `json:"data"`
}

// EventEmitter ...
type EventEmitter interface {
	Send(e Event) error
}

// Marshal returns the JSON encoding of Event.
func (e *Event) Marshal() (string, error) {
	b, err := json.Marshal(e)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
