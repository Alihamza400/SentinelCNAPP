module github.com/sentinel-cnapp/sentinel-cnapp/services/remediation

go 1.22.0

require (
	github.com/sentinel-cnapp/sentinel-cnapp v0.0.0
	github.com/aws/aws-sdk-go-v2 v1.27.0
	github.com/aws/aws-sdk-go-v2/config v1.27.0
	github.com/aws/aws-sdk-go-v2/service/s3 v1.54.0
	github.com/aws/aws-sdk-go-v2/service/ec2 v1.156.0
	github.com/aws/aws-sdk-go-v2/service/iam v1.33.0
	github.com/aws/aws-sdk-go-v2/service/ecr v1.28.0
	github.com/neo4j/neo4j-go-driver/v5 v5.20.0
	github.com/nats-io/nats.go v1.36.0
	github.com/prometheus/client_golang v1.19.1
)

replace github.com/sentinel-cnapp/sentinel-cnapp => ../..
