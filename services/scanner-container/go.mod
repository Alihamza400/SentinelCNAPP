module github.com/sentinel-cnapp/sentinel-cnapp/services/scanner-container

go 1.22.0

require (
	github.com/sentinel-cnapp/sentinel-cnapp v0.0.0
	github.com/nats-io/nats.go v1.36.0
	github.com/aws/aws-sdk-go-v2 v1.27.0
	github.com/aws/aws-sdk-go-v2/service/ecr v1.28.0
)

replace github.com/sentinel-cnapp/sentinel-cnapp => ../..
