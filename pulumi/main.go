package main

import (
	"fmt"
	"os"
	"path"

	"github.com/pulumi/pulumi-aws/sdk/v2/go/aws/iam"
	"github.com/pulumi/pulumi-aws/sdk/v2/go/aws/lambda"
	"github.com/pulumi/pulumi-aws/sdk/v2/go/aws/sqs"
	"github.com/pulumi/pulumi/sdk/v2/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v2/go/pulumi/config"
	"github.com/retgits/pulumi-helpers/v2/builder"
	"github.com/retgits/pulumi-helpers/v2/sampolicies"
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

// GenericConfig contains the key-value pairs for the configuration of AWS in this stack
type GenericConfig struct {
	// The AWS region used
	Region string

	// The DSN used to connect to Sentry
	SentryDSN string `json:"sentrydsn"`

	// The AWS AccountID to use
	AccountID string `json:"accountid"`

	// WavefrontURL is the URL of your Wavefront instance
	WavefrontURL string `json:"wavefronturl"`

	// WavefrontToken is your Wavefront API token
	WavefrontToken string `json:"wavefronttoken"`
}

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		// Get the region
		region, found := ctx.GetConfig("aws:region")
		if !found {
			return fmt.Errorf("region not found")
		}

		// Read the configuration data from Pulumi.<stack>.yaml
		conf := config.New(ctx, "awsconfig")

		// Create a new Tags object with the data from the configuration
		var tags Tags
		conf.RequireObject("tags", &tags)

		// Create a new GenericConfig object with the data from the configuration
		var genericConfig GenericConfig
		conf.RequireObject("generic", &genericConfig)
		genericConfig.Region = region

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
		buildFactory := builder.NewFactory().WithFolder(fnFolder)
		buildFactory.MustBuild()
		buildFactory.MustZip()

		// Create the IAM policy for the function.
		roleArgs := &iam.RoleArgs{
			AssumeRolePolicy: pulumi.String(sampolicies.AssumeRoleLambda()),
			Description:      pulumi.String("Role for the Shipment Service of the ACME Serverless Fitness Shop"),
			Tags:             pulumi.Map(tagMap),
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

		// Lookup the SQS queues
		responseQueue, err := sqs.LookupQueue(ctx, &sqs.LookupQueueArgs{
			Name: fmt.Sprintf("%s-acmeserverless-sqs-shipment-response", ctx.Stack()),
		})
		if err != nil {
			return err
		}

		requestQueue, err := sqs.LookupQueue(ctx, &sqs.LookupQueueArgs{
			Name: fmt.Sprintf("%s-acmeserverless-sqs-shipment-request", ctx.Stack()),
		})
		if err != nil {
			return err
		}

		// Create a factory to get policies from
		iamFactory := sampolicies.NewFactory().WithAccountID(genericConfig.AccountID).WithPartition("aws").WithRegion(genericConfig.Region)

		// Add a policy document to allow the function to use SQS as event source
		iamFactory.AddSQSSendMessagePolicy(responseQueue.Name)
		iamFactory.AddSQSPollerPolicy(requestQueue.Name)
		policies, err := iamFactory.GetPolicyStatement()
		if err != nil {
			return err
		}

		_, err = iam.NewRolePolicy(ctx, "ACMEServerlessShipmentSQSPolicy", &iam.RolePolicyArgs{
			Name:   pulumi.String("ACMEServerlessShipmentSQSPolicy"),
			Role:   role.Name,
			Policy: pulumi.String(policies),
		})
		if err != nil {
			return err
		}

		// Create the environment variables for the Lambda function
		variables := make(map[string]pulumi.StringInput)
		variables["REGION"] = pulumi.String(genericConfig.Region)
		variables["SENTRY_DSN"] = pulumi.String(genericConfig.SentryDSN)
		variables["FUNCTION_NAME"] = pulumi.String(fmt.Sprintf("%s-lambda-shipment", ctx.Stack()))
		variables["VERSION"] = tags.Version
		variables["STAGE"] = pulumi.String(ctx.Stack())
		variables["RESPONSEQUEUE"] = pulumi.String(responseQueue.Arn)
		variables["WAVEFRONT_URL"] = pulumi.String(genericConfig.WavefrontURL)
		variables["WAVEFRONT_API_TOKEN"] = pulumi.String(genericConfig.WavefrontToken)

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
			EventSourceArn: pulumi.String(requestQueue.Arn),
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
