package discovery

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	"github.com/sentinel-cnapp/sentinel-cnapp/services/asset-inventory/internal/store"
)

// EC2Discoverer discovers EC2 instances and related resources.
type EC2Discoverer struct {
	client *ec2.Client
	accountID string
}

// NewEC2Discoverer creates a new EC2 discoverer.
func NewEC2Discoverer(cfg aws.Config, accountID string) *EC2Discoverer {
	return &EC2Discoverer{
		client:    ec2.NewFromConfig(cfg),
		accountID: accountID,
	}
}

func (d *EC2Discoverer) Provider() string { return "aws" }
func (d *EC2Discoverer) Service() string  { return "ec2" }

func (d *EC2Discoverer) Discover(ctx context.Context, region string) ([]store.Asset, error) {
	var assets []store.Asset

	// Discover EC2 instances
	instances, err := d.discoverInstances(ctx, region)
	if err != nil {
		return nil, fmt.Errorf("discovering instances: %w", err)
	}
	assets = append(assets, instances...)

	// Discover EBS volumes
	volumes, err := d.discoverVolumes(ctx, region)
	if err != nil {
		return nil, fmt.Errorf("discovering volumes: %w", err)
	}
	assets = append(assets, volumes...)

	// Discover security groups
	sgs, err := d.discoverSecurityGroups(ctx, region)
	if err != nil {
		return nil, fmt.Errorf("discovering security groups: %w", err)
	}
	assets = append(assets, sgs...)

	// Discover VPCs
	vpcs, err := d.discoverVPCs(ctx, region)
	if err != nil {
		return nil, fmt.Errorf("discovering vpcs: %w", err)
	}
	assets = append(assets, vpcs...)

	return assets, nil
}

func (d *EC2Discoverer) discoverInstances(ctx context.Context, region string) ([]store.Asset, error) {
	var assets []store.Asset
	paginator := ec2.NewDescribeInstancesPaginator(d.client, &ec2.DescribeInstancesInput{})

	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}

		for _, reservation := range output.Reservations {
			for _, instance := range reservation.Instances {
				asset := d.instanceToAsset(instance, region)
				assets = append(assets, asset)
			}
		}
	}

	return assets, nil
}

func (d *EC2Discoverer) instanceToAsset(instance ec2types.Instance, region string) store.Asset {
	id := fmt.Sprintf("arn:aws:ec2:%s:%s:instance/%s", region, d.accountID, *instance.InstanceId)
	tags := ec2TagsToMap(instance.Tags)

	publicIP := ""
	if instance.PublicIpAddress != nil {
		publicIP = *instance.PublicIpAddress
	}

	privateIP := ""
	if instance.PrivateIpAddress != nil {
		privateIP = *instance.PrivateIpAddress
	}

	metadata := map[string]string{
		"instance_type": string(instance.InstanceType),
		"state":         string(instance.State.Name),
		"public_ip":     publicIP,
		"private_ip":    privateIP,
		"vpc_id":        safeString(instance.VpcId),
		"subnet_id":     safeString(instance.SubnetId),
		"launch_time":   instance.LaunchTime.Format(time.RFC3339),
	}
	metadataJSON := marshalMetadata(metadata)

	name := tags["Name"]
	if name == "" {
		name = *instance.InstanceId
	}

	now := time.Now().UTC()
	return store.Asset{
		ID:             id,
		Provider:       "aws",
		AssetType:      "ec2_instance",
		Name:           name,
		Region:         region,
		Tags:           tags,
		MetadataJSON:   metadataJSON,
		InternetFacing: publicIP != "",
		Environment:    inferEnvironment(tags),
		AccountID:      d.accountID,
		Active:         instance.State.Name != ec2types.InstanceStateNameTerminated,
		DiscoveredAt:   now,
		LastSyncedAt:   now,
	}
}

func (d *EC2Discoverer) discoverVolumes(ctx context.Context, region string) ([]store.Asset, error) {
	var assets []store.Asset
	paginator := ec2.NewDescribeVolumesPaginator(d.client, &ec2.DescribeVolumesInput{})

	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}

		for _, volume := range output.Volumes {
			asset := d.volumeToAsset(volume, region)
			assets = append(assets, asset)
		}
	}
	return assets, nil
}

func (d *EC2Discoverer) volumeToAsset(volume ec2types.Volume, region string) store.Asset {
	id := fmt.Sprintf("arn:aws:ec2:%s:%s:volume/%s", region, d.accountID, *volume.VolumeId)
	tags := ec2TagsToMap(volume.Tags)

	metadata := map[string]string{
		"size":      fmt.Sprintf("%d", volume.Size),
		"state":     string(volume.State),
		"volume_type": string(volume.VolumeType),
	}
	metadataJSON := marshalMetadata(metadata)

	now := time.Now().UTC()
	return store.Asset{
		ID:           id,
		Provider:     "aws",
		AssetType:    "ebs_volume",
		Name:         tags["Name"],
		Region:       region,
		Tags:         tags,
		MetadataJSON: metadataJSON,
		Environment:  inferEnvironment(tags),
		AccountID:    d.accountID,
		Active:       volume.State != ec2types.VolumeStateDeleting && volume.State != ec2types.VolumeStateDeleted,
		DiscoveredAt: now,
		LastSyncedAt: now,
	}
}

func (d *EC2Discoverer) discoverSecurityGroups(ctx context.Context, region string) ([]store.Asset, error) {
	var assets []store.Asset
	paginator := ec2.NewDescribeSecurityGroupsPaginator(d.client, &ec2.DescribeSecurityGroupsInput{})

	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}

		for _, sg := range output.SecurityGroups {
			id := fmt.Sprintf("arn:aws:ec2:%s:%s:security-group/%s", region, d.accountID, *sg.GroupId)
			tags := ec2TagsToMap(sg.Tags)

			// Check if security group allows public inbound access
			internetFacing := false
			for _, perm := range sg.IpPermissions {
				for _, ipRange := range perm.IpRanges {
					if ipRange.CidrIp != nil && *ipRange.CidrIp == "0.0.0.0/0" {
						internetFacing = true
						break
					}
				}
			}

			metadata := map[string]string{
				"group_name": safeString(sg.GroupName),
				"vpc_id":     safeString(sg.VpcId),
				"description": safeString(sg.Description),
			}
			metadataJSON := marshalMetadata(metadata)

			now := time.Now().UTC()
			assets = append(assets, store.Asset{
				ID:             id,
				Provider:       "aws",
				AssetType:      "security_group",
				Name:           tags["Name"],
				Region:         region,
				Tags:           tags,
				MetadataJSON:   metadataJSON,
				InternetFacing: internetFacing,
				Environment:    inferEnvironment(tags),
				AccountID:      d.accountID,
				Active:         true,
				DiscoveredAt:   now,
				LastSyncedAt:   now,
			})
		}
	}
	return assets, nil
}

func (d *EC2Discoverer) discoverVPCs(ctx context.Context, region string) ([]store.Asset, error) {
	var assets []store.Asset
	paginator := ec2.NewDescribeVpcsPaginator(d.client, &ec2.DescribeVpcsInput{})

	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}

		for _, vpc := range output.Vpcs {
			id := fmt.Sprintf("arn:aws:ec2:%s:%s:vpc/%s", region, d.accountID, *vpc.VpcId)
			tags := ec2TagsToMap(vpc.Tags)

			isDefault := false
			if vpc.IsDefault != nil {
				isDefault = *vpc.IsDefault
			}

			metadata := map[string]string{
				"is_default": fmt.Sprintf("%t", isDefault),
				"cidr":       safeString(vpc.CidrBlock),
				"state":      string(vpc.State),
			}
			metadataJSON := marshalMetadata(metadata)

			now := time.Now().UTC()
			assets = append(assets, store.Asset{
				ID:           id,
				Provider:     "aws",
				AssetType:    "vpc",
				Name:         tags["Name"],
				Region:       region,
				Tags:         tags,
				MetadataJSON: metadataJSON,
				Environment:  inferEnvironment(tags),
				AccountID:    d.accountID,
				Active:       vpc.State == ec2types.VpcStateAvailable,
				DiscoveredAt: now,
				LastSyncedAt: now,
			})
		}
	}
	return assets, nil
}
