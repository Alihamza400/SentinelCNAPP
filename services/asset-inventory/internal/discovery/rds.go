package discovery

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	rdsTypes "github.com/aws/aws-sdk-go-v2/service/rds/types"

	"github.com/sentinel-cnapp/sentinel-cnapp/services/asset-inventory/internal/store"
)

// RDSDiscoverer discovers RDS instances.
type RDSDiscoverer struct {
	client    *rds.Client
	accountID string
}

// NewRDSDiscoverer creates a new RDS discoverer.
func NewRDSDiscoverer(cfg aws.Config, accountID string) *RDSDiscoverer {
	return &RDSDiscoverer{
		client:    rds.NewFromConfig(cfg),
		accountID: accountID,
	}
}

func (d *RDSDiscoverer) Provider() string { return "aws" }
func (d *RDSDiscoverer) Service() string  { return "rds" }

func (d *RDSDiscoverer) Discover(ctx context.Context, region string) ([]store.Asset, error) {
	var assets []store.Asset
	paginator := rds.NewDescribeDBInstancesPaginator(d.client, &rds.DescribeDBInstancesInput{})

	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("describing RDS instances: %w", err)
		}

		for _, db := range output.DBInstances {
			asset := d.instanceToAsset(db, region)
			assets = append(assets, asset)
		}
	}

	return assets, nil
}

func (d *RDSDiscoverer) instanceToAsset(db rdsTypes.DBInstance, region string) store.Asset {
	id := fmt.Sprintf("arn:aws:rds:%s:%s:db:%s", region, d.accountID, aws.ToString(db.DBInstanceIdentifier))
	tags := rdsTagsToMap(db.TagList)

	// Check if publicly accessible
	internetFacing := false
	if db.PubliclyAccessible != nil {
		internetFacing = *db.PubliclyAccessible
	}

	engine := ""
	if db.Engine != nil {
		engine = *db.Engine
	}

	endpoint := ""
	if db.Endpoint != nil {
		endpoint = fmt.Sprintf("%s:%d", safeString(db.Endpoint.Address), aws.ToInt32(db.Endpoint.Port))
	}

	vpcID := ""
	if db.DBSubnetGroup != nil {
		vpcID = safeString(db.DBSubnetGroup.VpcId)
	}

	metadata := map[string]string{
		"engine":         engine,
		"engine_version": safeString(db.EngineVersion),
		"instance_class": safeString(db.DBInstanceClass),
		"status":         safeString(db.DBInstanceStatus),
		"endpoint":       endpoint,
		"storage":        fmt.Sprintf("%d", aws.ToInt32(db.AllocatedStorage)),
		"multi_az":       fmt.Sprintf("%t", aws.ToBool(db.MultiAZ)),
		"vpc_id":         vpcID,
	}
	metadataJSON := marshalMetadata(metadata)

	now := time.Now().UTC()
	return store.Asset{
		ID:             id,
		Provider:       "aws",
		AssetType:      "rds_instance",
		Name:           aws.ToString(db.DBInstanceIdentifier),
		Region:         region,
		Tags:           tags,
		MetadataJSON:   metadataJSON,
		InternetFacing: internetFacing,
		Environment:    inferEnvironment(tags),
		AccountID:      d.accountID,
		Active:         db.DBInstanceStatus != nil && *db.DBInstanceStatus == "available",
		DiscoveredAt:   now,
		LastSyncedAt:   now,
	}
}
