package discovery

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecr"
	ecrTypes "github.com/aws/aws-sdk-go-v2/service/ecr/types"

	"github.com/sentinel-cnapp/sentinel-cnapp/services/asset-inventory/internal/store"
)

// ECRDiscoverer discovers ECR repositories.
type ECRDiscoverer struct {
	client    *ecr.Client
	accountID string
}

// NewECRDiscoverer creates a new ECR discoverer.
func NewECRDiscoverer(cfg aws.Config, accountID string) *ECRDiscoverer {
	return &ECRDiscoverer{
		client:    ecr.NewFromConfig(cfg),
		accountID: accountID,
	}
}

func (d *ECRDiscoverer) Provider() string { return "aws" }
func (d *ECRDiscoverer) Service() string  { return "ecr" }

func (d *ECRDiscoverer) Discover(ctx context.Context, region string) ([]store.Asset, error) {
	var assets []store.Asset
	paginator := ecr.NewDescribeRepositoriesPaginator(d.client, &ecr.DescribeRepositoriesInput{})

	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("describing ECR repositories: %w", err)
		}

		for _, repo := range output.Repositories {
			asset := d.repoToAsset(ctx, repo, region)
			assets = append(assets, asset)
		}
	}

	return assets, nil
}

func (d *ECRDiscoverer) repoToAsset(ctx context.Context, repo ecrTypes.Repository, region string) store.Asset {
	id := fmt.Sprintf("arn:aws:ecr:%s:%s:repository/%s", region, d.accountID, aws.ToString(repo.RepositoryName))

	// ECR DescribeRepositories does not include tags; fetch them separately.
	tags := make(map[string]string)
	if repo.RepositoryArn != nil {
		if tagOut, err := d.client.ListTagsForResource(ctx, &ecr.ListTagsForResourceInput{
			ResourceArn: repo.RepositoryArn,
		}); err == nil {
			tags = ecrTagsToMap(tagOut.Tags)
		}
	}

	createdAt := ""
	if repo.CreatedAt != nil {
		createdAt = repo.CreatedAt.Format(time.RFC3339)
	}

	metadata := map[string]string{
		"repository_uri":        safeString(repo.RepositoryUri),
		"arn":                   safeString(repo.RepositoryArn),
		"created_at":            createdAt,
		"image_tag_mutability":  string(repo.ImageTagMutability),
		"scan_on_push":          fmt.Sprintf("%t", repo.ImageScanningConfiguration != nil && repo.ImageScanningConfiguration.ScanOnPush),
	}
	metadataJSON := marshalMetadata(metadata)

	// Check if repository has a policy that could expose it publicly.
	internetFacing := false
	policyResult, err := d.client.GetRepositoryPolicy(ctx, &ecr.GetRepositoryPolicyInput{
		RepositoryName: repo.RepositoryName,
	})
	if err == nil && policyResult.PolicyText != nil {
		internetFacing = true
	}

	now := time.Now().UTC()
	return store.Asset{
		ID:             id,
		Provider:       "aws",
		AssetType:      "ecr_repository",
		Name:           aws.ToString(repo.RepositoryName),
		Region:         region,
		Tags:           tags,
		MetadataJSON:   metadataJSON,
		InternetFacing: internetFacing,
		Environment:    inferEnvironment(tags),
		AccountID:      d.accountID,
		Active:         true,
		DiscoveredAt:   now,
		LastSyncedAt:   now,
	}
}
