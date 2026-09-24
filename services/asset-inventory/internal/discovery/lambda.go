package discovery

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	lambdaTypes "github.com/aws/aws-sdk-go-v2/service/lambda/types"

	"github.com/sentinel-cnapp/sentinel-cnapp/services/asset-inventory/internal/store"
)

// LambdaDiscoverer discovers Lambda functions.
type LambdaDiscoverer struct {
	client    *lambda.Client
	accountID string
}

// NewLambdaDiscoverer creates a new Lambda discoverer.
func NewLambdaDiscoverer(cfg aws.Config, accountID string) *LambdaDiscoverer {
	return &LambdaDiscoverer{
		client:    lambda.NewFromConfig(cfg),
		accountID: accountID,
	}
}

func (d *LambdaDiscoverer) Provider() string { return "aws" }
func (d *LambdaDiscoverer) Service() string  { return "lambda" }

func (d *LambdaDiscoverer) Discover(ctx context.Context, region string) ([]store.Asset, error) {
	var assets []store.Asset
	paginator := lambda.NewListFunctionsPaginator(d.client, &lambda.ListFunctionsInput{})

	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("listing Lambda functions: %w", err)
		}

		for _, fn := range output.Functions {
			asset := d.functionToAsset(fn, region)
			assets = append(assets, asset)
		}
	}

	return assets, nil
}

func (d *LambdaDiscoverer) functionToAsset(fn lambdaTypes.FunctionConfiguration, region string) store.Asset {
	id := fmt.Sprintf("arn:aws:lambda:%s:%s:function:%s", region, d.accountID, aws.ToString(fn.FunctionName))
	tags := make(map[string]string)

	runtime := ""
	if fn.Runtime != "" {
		runtime = string(fn.Runtime)
	}

	metadata := map[string]string{
		"runtime":       runtime,
		"handler":       safeString(fn.Handler),
		"memory":        fmt.Sprintf("%d", aws.ToInt32(fn.MemorySize)),
		"timeout":       fmt.Sprintf("%d", aws.ToInt32(fn.Timeout)),
		"last_modified": safeString(fn.LastModified),
		"code_size":     fmt.Sprintf("%d", fn.CodeSize),
		"role":          safeString(fn.Role),
	}

	// Check if Lambda has a VPC config (internet access)
	if fn.VpcConfig != nil {
		metadata["vpc_id"] = safeString(fn.VpcConfig.VpcId)
		metadata["vpc_subnets"] = fmt.Sprintf("%v", fn.VpcConfig.SubnetIds)
	}

	metadataJSON := marshalMetadata(metadata)

	now := time.Now().UTC()
	return store.Asset{
		ID:           id,
		Provider:     "aws",
		AssetType:    "lambda_function",
		Name:         aws.ToString(fn.FunctionName),
		Region:       region,
		Tags:         tags,
		MetadataJSON: metadataJSON,
		Environment:  inferEnvironment(tags),
		AccountID:    d.accountID,
		Active:       true,
		DiscoveredAt: now,
		LastSyncedAt: now,
	}
}
