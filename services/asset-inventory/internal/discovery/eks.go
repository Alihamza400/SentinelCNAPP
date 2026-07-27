package discovery

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	eksTypes "github.com/aws/aws-sdk-go-v2/service/eks/types"

	"github.com/sentinel-cnapp/sentinel-cnapp/services/asset-inventory/internal/store"
)

// EKSDiscoverer discovers EKS clusters.
type EKSDiscoverer struct {
	client    *eks.Client
	accountID string
}

// NewEKSDiscoverer creates a new EKS discoverer.
func NewEKSDiscoverer(cfg aws.Config, accountID string) *EKSDiscoverer {
	return &EKSDiscoverer{
		client:    eks.NewFromConfig(cfg),
		accountID: accountID,
	}
}

func (d *EKSDiscoverer) Provider() string { return "aws" }
func (d *EKSDiscoverer) Service() string  { return "eks" }

func (d *EKSDiscoverer) Discover(ctx context.Context, region string) ([]store.Asset, error) {
	var assets []store.Asset

	paginator := eks.NewListClustersPaginator(d.client, &eks.ListClustersInput{})

	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("listing EKS clusters: %w", err)
		}

		for _, clusterName := range output.Clusters {
			cluster, err := d.client.DescribeCluster(ctx, &eks.DescribeClusterInput{
				Name: aws.String(clusterName),
			})
			if err != nil {
				return nil, fmt.Errorf("describing cluster %s: %w", clusterName, err)
			}

			asset := d.clusterToAsset(cluster.Cluster, region)
			assets = append(assets, asset)
		}
	}

	return assets, nil
}

func (d *EKSDiscoverer) clusterToAsset(cluster *eksTypes.Cluster, region string) store.Asset {
	id := fmt.Sprintf("arn:aws:eks:%s:%s:cluster/%s", region, d.accountID, *cluster.Name)
	tags := awsTagsToMap(cluster.Tags)

	metadata := map[string]string{
		"version":           safeString(cluster.Version),
		"status":            string(cluster.Status),
		"endpoint":          safeString(cluster.Endpoint),
		"role_arn":          safeString(cluster.RoleArn),
		"security_group_id": safeString(cluster.ResourcesVpcConfig.ClusterSecurityGroupId),
		"subnet_ids":        fmt.Sprintf("%v", cluster.ResourcesVpcConfig.SubnetIds),
		"vpc_id":            safeString(cluster.ResourcesVpcConfig.VpcId),
		"platform_version":  safeString(cluster.PlatformVersion),
	}
	metadataJSON := marshalMetadata(metadata)

	now := time.Now().UTC()
	return store.Asset{
		ID:           id,
		Provider:     "aws",
		AssetType:    "eks_cluster",
		Name:         *cluster.Name,
		Region:       region,
		Tags:         tags,
		MetadataJSON: metadataJSON,
		Environment:  inferEnvironment(tags),
		AccountID:    d.accountID,
		Active:       cluster.Status == eksTypes.ClusterStatusActive,
		DiscoveredAt: now,
		LastSyncedAt: now,
	}
}
