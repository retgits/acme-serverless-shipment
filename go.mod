module github.com/retgits/shipment

go 1.13

require (
	github.com/aws/aws-lambda-go v1.13.2
	github.com/aws/aws-sdk-go v1.25.31
	github.com/gofrs/uuid v3.2.0+incompatible
	github.com/kelseyhightower/envconfig v1.4.0
	github.com/rcrowley/go-metrics v0.0.0-20190826022208-cac0b30c2563 // indirect
	github.com/retgits/creditcard v0.2.0
	github.com/wavefronthq/wavefront-lambda-go v0.0.0-20191029210830-5fe579f2b811
)

replace github.com/wavefronthq/wavefront-lambda-go => /Users/lstigter/repos/github.com/retgits/wavefront-lambda-go
