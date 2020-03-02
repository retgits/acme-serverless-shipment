# Shipment

> A shipping service, because what is a shop without a way to ship your purchases?

The Shipping service is part of the [ACME Fitness Serverless Shop](https://github.com/retgits/acme-serverless). The goal of this specific service is, as the name implies, to ship products using a wide variety of shipping suppliers.

## Prerequisites

* [Go (at least Go 1.12)](https://golang.org/dl/)
* [An AWS Account](https://portal.aws.amazon.com/billing/signup)
* The _vuln_ targets for Make and Mage rely on the [Snyk](http://snyk.io/) CLI
* This service uses [Sentry.io](https://sentry.io) for tracing and error reporting

## Eventing Options

The shipment service has a few different eventing platforms available:

* [Amazon EventBridge](https://aws.amazon.com/eventbridge/)
* [Amazon Simple Queue Service](https://aws.amazon.com/sqs/)

For all options there is a Lambda function ready to be deployed where events arrive from and are sent to that particular eventing mechanism. You can find the code in `./cmd/lambda-<event platform>`. There is a ready made test function available as well that sends a message to the eventing platform. The code for the tester can be found in `./cmd/test-<event platform>`. The messages the testing app sends, are located under the [`test`](./test) folder.

## Using Amazon EventBridge

### Prerequisites for EventBridge

* [AWS CLI](https://docs.aws.amazon.com/cli/latest/userguide/cli-chap-install.html) installed and configured
* [Custom EventBus](https://docs.aws.amazon.com/eventbridge/latest/userguide/create-event-bus.html) configured, the name of the configured event bus should be set as the `feature` parameter in the `template.yaml` file.

### Build and deploy for EventBridge

Clone this repository

```bash
git clone https://github.com/retgits/acme-serverless-shipment
cd acme-serverless-shipment
```

Get the Go Module dependencies

```bash
go get ./...
```

Change directories to the [deploy/cloudformation](./deploy/cloudformation) folder

```bash
cd ./deploy/cloudformation
```

If your event bus is not called _acmeserverless_, update the name of the `feature` parameter in the `template.yaml` file. Now you can build and deploy the Lambda function:

```bash
make build TYPE=eventbridge
make deploy TYPE=eventbridge
```

### Testing EventBridge

To send a message to an Amazon EventBridge eventbus, check out the [acme-serverless README](https://github.com/retgits/acme-serverless#testing-eventbridge)

### Prerequisites for SQS

* [AWS CLI](https://docs.aws.amazon.com/cli/latest/userguide/cli-chap-install.html) installed and configured

### Build and deploy for SQS

Clone this repository

```bash
git clone https://github.com/retgits/acme-serverless-shipment
cd acme-serverless-shipment
```

Get the Go Module dependencies

```bash
go get ./...
```

Change directories to the [deploy/cloudformation](./deploy/cloudformation) folder

```bash
cd ./deploy/cloudformation
```

Now you can build and deploy the Lambda function:

```bash
make build TYPE=sqs
make deploy TYPE=sqs
```

### Testing SQS

To send a message to an SQS queue, check out the [acme-serverless README](https://github.com/retgits/acme-serverless#testing-sqs)

## Events

The events for all of ACME Serverless Fitness Shop are structured as

```json
{
    "metadata": { // Metadata for all services
        "domain": "Order", // Domain represents the the event came from (like Payment or Order)
        "source": "CLI", // Source represents the function the event came from (like ValidateCreditCard or SubmitOrder)
        "type": "ShipmentRequested", // Type respresents the type of event this is (like CreditCardValidated)
        "status": "success" // Status represents the current status of the event (like Success)
    },
    "data": {} // The actual payload of the event
}
```

The input that the function expects, either as the direct message or after transforming the JSON is

```json
{
    "metadata": {
        "domain": "Order",
        "source": "CLI",
        "type": "ShipmentRequested",
        "status": "success"
    },
    "data": {
        "_id": "1234",
        "delivery": "UPS/FEDEX"
    }
}
```

A successful shipment results in two events being sent. The first event after the shipment has been sent

```json
{
    "metadata": {
        "domain": "Shipment",
        "source": "SendShipment",
        "type": "SentShipment",
        "status": "success"
    },
    "data": {
        "trackingNumber": "6bc3b96b-19e9-42d3-bff8-91a2c431d1d6",
        "orderNumber": "12345",
        "status": "shipped - pending delivery"
    }
}
```

The second event is sent after delivery

```json
{
    "metadata": {
        "domain": "Shipment",
        "source": "SendShipment",
        "type": "DeliveredShipment",
        "status": "success"
    },
    "data": {
        "trackingNumber": "6bc3b96b-19e9-42d3-bff8-91a2c431d1d6",
        "orderNumber": "12345",
        "status": "delivered"
    }
}
```

## Using Make

The Makefiles for the Lambda executables have a few a bunch of options available:

| Target  | Description                                                |
|---------|------------------------------------------------------------|
| build   | Build the executable for Lambda                            |
| clean   | Remove all generated files                                 |
| deploy  | Deploy the app to AWS Lambda                               |
| destroy | Deletes the CloudFormation stack and all created resources |
| help    | Displays the help for each target (this message)           |
| vuln    | Scans the Go.mod file for known vulnerabilities using Snyk |

The targets `build` and `deploy` need a variable **`TYPE`** set to either `eventbridge` or `sqs` to build and deploy the correct Lambda functions.

## Using Mage

If you want to "go all Go" (_pun intended_) and write plain-old go functions to build and deploy, you can use [Mage](https://magefile.org/). Mage is a make/rake-like build tool using Go so Mage automatically uses the functions you create as Makefile-like runnable targets.

### Prerequisites for Mage

To use Mage, you'll need to install it first:

```bash
go get -u -d github.com/magefile/mage
cd $GOPATH/src/github.com/magefile/mage
go run bootstrap.go
```

Instructions curtesy of Mage

### Targets

The Magefile in this repository has a bunch of targets available:

| Target | Description                                                                                              |
|--------|----------------------------------------------------------------------------------------------------------|
| build  | compiles the individual commands in the cmd folder, along with their dependencies.                       |
| clean  | removes object files from package source directories.                                                    |
| deploy | packages, deploys, and returns all outputs of your stack.                                                |
| deps   | resolves and downloads dependencies to the current development module and then builds and installs them. |
| test   | 'Go test' automates testing the packages named by the import paths.                                      |
| vuln   | uses Snyk to test for any known vulnerabilities in go.mod.                                               |

The targets `build` and `deploy` need a variable **`TYPE`** set to either `eventbridge` or `sqs` to build and deploy the correct Lambda functions.

## Contributing

[Pull requests](https://github.com/retgits/acme-serverless-shipment/pulls) are welcome. For major changes, please open [an issue](https://github.com/retgits/acme-serverless-shipment/issues) first to discuss what you would like to change.

Please make sure to update tests as appropriate.

## License

See the [LICENSE](./LICENSE) file in the repository
