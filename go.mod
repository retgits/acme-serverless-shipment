module github.com/retgits/shipment

go 1.13

require (
	github.com/aws/aws-lambda-go v1.13.2
	github.com/aws/aws-sdk-go v1.25.31
	github.com/gofrs/uuid v3.2.0+incompatible
	github.com/kelseyhightower/envconfig v1.4.0
	github.com/leesper/go_rng v0.0.0-20190531154944-a612b043e353 // indirect
	github.com/stretchr/testify v1.4.0
	github.com/wavefronthq/wavefront-lambda-go v0.0.0-20191029210830-5fe579f2b811
	golang.org/x/net v0.0.0-20191112182307-2180aed22343 // indirect
	gonum.org/v1/gonum v0.6.0 // indirect
)

replace github.com/wavefronthq/wavefront-lambda-go => /Users/lstigter/repos/github.com/retgits/wavefront-lambda-go
