package main

import (
	"fmt"
	"os"
	"os/exec"
	"path"

	"github.com/pulumi/pulumi-aws/sdk/go/aws/iam"
	"github.com/pulumi/pulumi-aws/sdk/go/aws/lambda"
	"github.com/pulumi/pulumi/sdk/go/pulumi"
	"github.com/pulumi/pulumi/sdk/go/pulumi/config"
)

const (
	// The shell to use
	shell = "sh"

	// The flag for the shell to read commands from a string
	shellFlag = "-c"
)

// Tags are key-value pairs to apply to the resources created by this stack
type Tags struct {
	// Author is the person who created the code, or performed the deployment
	Author pulumi.String

	// Feature is the project that this resource belongs to
	Feature pulumi.String

	// Team is the team that is responsible to manage this resource
	Team pulumi.String

	// Version is the version of the code for this resource
	Version pulumi.String
}

// LambdaConfig contains the key-value pairs for the configuration of AWS Lambda in this stack
type LambdaConfig struct {
	// The SQS queue to send responses to
	ShipmentResponseQueue string `json:"responsequeue"`

	// The SQS queue to receives messages from
	ShipmentRequestQueue string `json:"requestqueue"`

	// The AWS region used
	Region string `json:"region"`

	// The DSN used to connect to Sentry
	SentryDSN string `json:"sentrydsn"`
}

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		// Read the configuration data from Pulumi.<stack>.yaml
		conf := config.New(ctx, "awsconfig")

		// Create a new Tags object with the data from the configuration
		var tags Tags
		conf.RequireObject("tags", &tags)

		// Create a new DynamoConfig object with the data from the configuration
		var lambdaConfig LambdaConfig
		conf.RequireObject("lambda", &lambdaConfig)

		// Create a map[string]pulumi.Input of the tags
		// the first four tags come from the configuration file
		// the last two are derived from this deployment
		tagMap := make(map[string]pulumi.Input)
		tagMap["Author"] = tags.Author
		tagMap["Feature"] = tags.Feature
		tagMap["Team"] = tags.Team
		tagMap["Version"] = tags.Version
		tagMap["ManagedBy"] = pulumi.String("Pulumi")
		tagMap["Stage"] = pulumi.String(ctx.Stack())

		// Compile and zip the AWS Lambda functions
		wd, err := os.Getwd()
		if err != nil {
			return err
		}

		// Find the working folder
		fnFolder := path.Join(wd, "..", "cmd", "lambda-shipment-sqs")

		// Run go build
		if err := run(fnFolder, "GOOS=linux GOARCH=amd64 go build"); err != nil {
			fmt.Printf("Error building code: %s", err.Error())
			os.Exit(1)
		}

		// Zip up the binary
		if err := run(fnFolder, "zip ./lambda-shipment-sqs.zip ./lambda-shipment-sqs"); err != nil {
			fmt.Printf("Error creating zipfile: %s", err.Error())
			os.Exit(1)
		}

		// Create the IAM policy for the function.
		roleArgs := &iam.RoleArgs{
			AssumeRolePolicy: pulumi.String(`{
				"Version": "2012-10-17",
				"Statement": [
				{
					"Action": "sts:AssumeRole",
					"Principal": {
						"Service": "lambda.amazonaws.com"
					},
					"Effect": "Allow",
					"Sid": ""
				}
				]
			}`),
			Description: pulumi.String("Role for the Shipment Service of the ACME Serverless Fitness Shop"),
			Tags:        pulumi.Map(tagMap),
		}

		role, err := iam.NewRole(ctx, "ACMEServerlessShipmentRole", roleArgs)
		if err != nil {
			return err
		}

		_, err = iam.NewRolePolicyAttachment(ctx, "AWSLambdaBasicExecutionRole", &iam.RolePolicyAttachmentArgs{
			PolicyArn: pulumi.String("arn:aws:iam::aws:policy/service-role/AWSLambdaBasicExecutionRole"),
			Role:      role.Name,
		})
		if err != nil {
			return err
		}

		// Add a policy document to allow the function to use SQS as event source
		policyString := fmt.Sprintf(`{
				"Version": "2012-10-17",
				"Statement": [
					{
						"Action": [
							"sqs:SendMessage*"
						],
						"Effect": "Allow",
						"Resource": "%s"
					},
					{
						"Action": [
							"sqs:ReceiveMessage",
							"sqs:DeleteMessage",
							"sqs:GetQueueAttributes"
						],
						"Effect": "Allow",
						"Resource": "%s"
					}
				]
			}`, lambdaConfig.ShipmentResponseQueue, lambdaConfig.ShipmentRequestQueue)

		_, err = iam.NewRolePolicy(ctx, "ACMEServerlessShipmentSQSPolicy", &iam.RolePolicyArgs{
			Name:   pulumi.String("ACMEServerlessShipmentSQSPolicy"),
			Role:   role.Name,
			Policy: pulumi.String(policyString),
		})
		if err != nil {
			return err
		}

		// Create the environment variables for the Lambda function
		variables := make(map[string]pulumi.StringInput)
		variables["REGION"] = pulumi.String(lambdaConfig.Region)
		variables["SENTRY_DSN"] = pulumi.String(lambdaConfig.SentryDSN)
		variables["FUNCTION_NAME"] = pulumi.String(fmt.Sprintf("%s-lambda-shipment", ctx.Stack()))
		variables["VERSION"] = tags.Version
		variables["STAGE"] = pulumi.String(ctx.Stack())
		variables["RESPONSEQUEUE"] = pulumi.String(lambdaConfig.ShipmentResponseQueue)

		environment := lambda.FunctionEnvironmentArgs{
			Variables: pulumi.StringMap(variables),
		}

		// Create the AWS Lambda function
		functionArgs := &lambda.FunctionArgs{
			Description: pulumi.String("A Lambda function to handle shipments"),
			Runtime:     pulumi.String("go1.x"),
			Name:        pulumi.String(fmt.Sprintf("%s-lambda-shipment", ctx.Stack())),
			MemorySize:  pulumi.Int(256),
			Timeout:     pulumi.Int(10),
			Handler:     pulumi.String("lambda-shipment-sqs"),
			Environment: environment,
			Code:        pulumi.NewFileArchive("../cmd/lambda-shipment-sqs/lambda-shipment-sqs.zip"),
			Role:        role.Arn,
			Tags:        pulumi.Map(tagMap),
		}

		function, err := lambda.NewFunction(ctx, fmt.Sprintf("%s-lambda-shipment", ctx.Stack()), functionArgs)
		if err != nil {
			return err
		}

		_, err = lambda.NewEventSourceMapping(ctx, fmt.Sprintf("%s-lambda-shipment", ctx.Stack()), &lambda.EventSourceMappingArgs{
			BatchSize:      pulumi.Int(1),
			Enabled:        pulumi.Bool(true),
			FunctionName:   function.Arn,
			EventSourceArn: pulumi.String(lambdaConfig.ShipmentRequestQueue),
		})
		if err != nil {
			return err
		}

		// Export the Role ARN and Function ARN as an output of the Pulumi stack
		ctx.Export("ACMEServerlessShipmentRole::Arn", role.Arn)
		ctx.Export("lambda-shipment-sqs::Arn", function.Arn)

		return nil
	})
}

// run creates a Cmd struct to execute the named program with the given arguments.
// After that, it starts the specified command and waits for it to complete.
func run(folder string, args string) error {
	cmd := exec.Command(shell, shellFlag, args)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Dir = folder
	return cmd.Run()
}
