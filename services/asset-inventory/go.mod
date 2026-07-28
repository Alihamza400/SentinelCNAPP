module github.com/sentinel-cnapp/sentinel-cnapp/services/asset-inventory

go 1.22.0

require (
	github.com/sentinel-cnapp/sentinel-cnapp v0.0.0
	github.com/aws/aws-sdk-go-v2 v1.27.0
	github.com/aws/aws-sdk-go-v2/config v1.27.0
	github.com/aws/aws-sdk-go-v2/service/ec2 v1.156.0
	github.com/aws/aws-sdk-go-v2/service/eks v1.42.0
	github.com/aws/aws-sdk-go-v2/service/iam v1.33.0
	github.com/aws/aws-sdk-go-v2/service/s3 v1.54.0
	github.com/aws/aws-sdk-go-v2/service/lambda v1.54.0
	github.com/aws/aws-sdk-go-v2/service/ecr v1.28.0
	github.com/aws/aws-sdk-go-v2/service/rds v1.78.0
	github.com/aws/aws-sdk-go-v2/service/sts v1.28.0
)

replace github.com/sentinel-cnapp/sentinel-cnapp => ../..
