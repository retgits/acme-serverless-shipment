package main

import (
	"encoding/json"
	"log"
	"math/rand"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sqs"
	"github.com/kelseyhightower/envconfig"

	uuid "github.com/gofrs/uuid"
	wflambda "github.com/wavefronthq/wavefront-lambda-go"
)

var wfAgent = wflambda.NewWavefrontAgent(&wflambda.WavefrontConfig{})

func UnmarshalOrder(data []byte) (Order, error) {
	var r Order
	err := json.Unmarshal(data, &r)
	return r, err
}

type Order struct {
	OrderID   string   `json:"_id"`
	UserID    *string  `json:"userid,omitempty"`
	Firstname *string  `json:"firstname,omitempty"`
	Lastname  *string  `json:"lastname,omitempty"`
	Address   *Address `json:"address,omitempty"`
	Email     *string  `json:"email,omitempty"`
	Delivery  string   `json:"delivery"`
	Card      *Card    `json:"card,omitempty"`
	Cart      []Cart   `json:"cart"`
	Total     *string  `json:"total,omitempty"`
}

type Address struct {
	Street  *string `json:"street,omitempty"`
	City    *string `json:"city,omitempty"`
	Zip     *string `json:"zip,omitempty"`
	State   *string `json:"state,omitempty"`
	Country *string `json:"country,omitempty"`
}

type Card struct {
	Type     *string `json:"type,omitempty"`
	Number   *string `json:"number,omitempty"`
	ExpMonth *string `json:"expMonth,omitempty"`
	ExpYear  *string `json:"expYear,omitempty"`
	Ccv      *string `json:"ccv,omitempty"`
}

type Cart struct {
	ID          *string `json:"id,omitempty"`
	Description *string `json:"description,omitempty"`
	Quantity    *string `json:"quantity,omitempty"`
	Price       *string `json:"price,omitempty"`
}

// Shipment contains all fields that the shipping service needs to work. The
// TrackingNumber is generated by the shipper, which also determines the Status.
// The OrderNumber comes from the OrderID field received from the orderservice.
type Shipment struct {
	TrackingNumber string `json:"trackingNumber"`
	OrderNumber    string `json:"orderNumber"`
	Status         string `json:"status"`
}

// Marshal takes a shipment and turns it into a byte array
func (r *Shipment) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

// config is the struct that is used to keep track of all environment variables
type config struct {
	AWSRegion     string `required:"true" split_words:"true" envconfig:"REGION"`
	ResponseQueue string `required:"true" split_words:"true" envconfig:"RESPONSE_QUEUE"`
}

// DeliveryTime generates a random number between the min and max values, to
// determine how long a go routine will sleep to simulate delivery. The min
// and max values are set during the initialization of the shipper.
func deliveryTime(min int, max int) int {
	rand.Seed(time.Now().UTC().UnixNano())
	return min + rand.Intn(max-min)
}

const (
	minDeliveryTime = 5
	maxDeliveryTime = 120
)

var c config

func handler(request events.SQSEvent) error {
	// Get configuration set using environment variables
	err := envconfig.Process("", &c)
	if err != nil {
		log.Printf("error starting function: %s", err.Error())
		return err
	}

	// Create an AWS session
	awsSession := session.Must(session.NewSession(&aws.Config{
		Region: aws.String(c.AWSRegion),
	}))

	// Create a SQS session
	sqsService := sqs.New(awsSession)

	for _, record := range request.Records {
		msg, err := UnmarshalOrder([]byte(record.Body))
		if err != nil {
			log.Printf("error unmarshaling request: %s", err.Error())
			break
		}

		log.Printf("Hello, this is %s... We'll take care of your package!", msg.Delivery)

		trackingnumber := uuid.Must(uuid.NewV4()).String()

		res := Shipment{
			TrackingNumber: trackingnumber,
			OrderNumber:    msg.OrderID,
			Status:         "shipped - pending delivery",
		}

		pl, _ := res.Marshal()

		sendMessageInput := &sqs.SendMessageInput{
			QueueUrl:    aws.String(c.ResponseQueue),
			MessageBody: aws.String(string(pl)),
		}

		_, err = sqsService.SendMessage(sendMessageInput)
		if err != nil {
			log.Printf("error while sending response message: %s", err.Error())
			break
		}

		d := deliveryTime(minDeliveryTime, maxDeliveryTime)
		time.Sleep(time.Duration(d) * time.Second)

		res.Status = "delivered"
		pl, _ = res.Marshal()

		sendMessageInput = &sqs.SendMessageInput{
			QueueUrl:    aws.String(c.ResponseQueue),
			MessageBody: aws.String(string(pl)),
		}

		_, err = sqsService.SendMessage(sendMessageInput)
		if err != nil {
			log.Printf("error while sending response message: %s", err.Error())
			break
		}
	}

	return nil
}

// The main method is executed by AWS Lambda and points to the handler
func main() {
	lambda.Start(wfAgent.WrapHandler(handler))
}