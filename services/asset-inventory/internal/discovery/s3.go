package discovery

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3Types "github.com/aws/aws-sdk-go-v2/service/s3/types"

	"github.com/sentinel-cnapp/sentinel-cnapp/services/asset-inventory/internal/store"
)

// S3Discoverer discovers S3 buckets and checks public access.
type S3Discoverer struct {
	client    *s3.Client
	accountID string
}

// NewS3Discoverer creates a new S3 discoverer.
func NewS3Discoverer(cfg aws.Config, accountID string) *S3Discoverer {
	return &S3Discoverer{
		client:    s3.NewFromConfig(cfg),
		accountID: accountID,
	}
}

func (d *S3Discoverer) Provider() string { return "aws" }
func (d *S3Discoverer) Service() string  { return "s3" }

func (d *S3Discoverer) Discover(ctx context.Context, region string) ([]store.Asset, error) {
	// S3 is a global service — ListBuckets doesn't take a region.
	// We list all buckets and filter by region.
	var allBuckets []s3Types.Bucket

	// We use the first region for listing; S3 is global
	result, err := d.client.ListBuckets(ctx, &s3.ListBucketsInput{})
	if err != nil {
		return nil, fmt.Errorf("listing S3 buckets: %w", err)
	}

	for _, bucket := range result.Buckets {
		// Get bucket location to filter by region
		locResult, err := d.client.GetBucketLocation(ctx, &s3.GetBucketLocationInput{
			Bucket: bucket.Name,
		})
		if err != nil {
			// Skip buckets we can't access
			continue
		}

		bucketRegion := string(locResult.LocationConstraint)
		// us-east-1 returns empty string
		if bucketRegion == "" {
			bucketRegion = "us-east-1"
		}

		if bucketRegion == region {
			allBuckets = append(allBuckets, bucket)
		}
	}

	var assets []store.Asset
	for _, bucket := range allBuckets {
		// Bucket names are global, so we can use it as the ID
		id := fmt.Sprintf("arn:aws:s3:::%s", *bucket.Name)

		// Get bucket tags
		tagResult, err := d.client.GetBucketTagging(ctx, &s3.GetBucketTaggingInput{
			Bucket: bucket.Name,
		})
		var tags map[string]string
		if err == nil {
			tags = s3TagsToMap(tagResult.TagSet)
		} else {
			tags = make(map[string]string)
		}

		// Check public access via bucket ACL
		internetFacing := d.checkPublicAccess(ctx, bucket.Name)

		metadata := map[string]string{
			"creation_date": bucket.CreationDate.Format(time.RFC3339),
		}

		now := time.Now().UTC()
		asset := store.Asset{
			ID:             id,
			Provider:       "aws",
			AssetType:      "s3_bucket",
			Name:           *bucket.Name,
			Region:         region,
			Tags:           tags,
			InternetFacing: internetFacing,
			Environment:    inferEnvironment(tags),
			AccountID:      d.accountID,
			Active:         true,
			DiscoveredAt:   now,
			LastSyncedAt:   now,
		}
		asset.MetadataJSON = marshalMetadata(metadata)

		assets = append(assets, asset)
	}

	return assets, nil
}

func (d *S3Discoverer) checkPublicAccess(ctx context.Context, bucketName *string) bool {
	// Check bucket ACL for public access
	aclResult, err := d.client.GetBucketAcl(ctx, &s3.GetBucketAclInput{
		Bucket: bucketName,
	})
	if err != nil {
		return false
	}

	for _, grant := range aclResult.Grants {
		if grant.Grantee == nil {
			continue
		}
		if grant.Grantee.URI != nil {
			// URI "http://acs.amazonaws.com/groups/global/AllUsers" or "AuthenticatedUsers"
			uri := *grant.Grantee.URI
			if uri == "http://acs.amazonaws.com/groups/global/AllUsers" ||
				uri == "http://acs.amazonaws.com/groups/global/AuthenticatedUsers" {
				return true
			}
		}
	}

	// Check bucket policy for public access
	policyResult, err := d.client.GetBucketPolicy(ctx, &s3.GetBucketPolicyInput{
		Bucket: bucketName,
	})
	if err == nil && policyResult.Policy != nil {
		// If a bucket policy exists, it might allow public access — mark as potentially exposed
		// Detailed policy parsing happens in CSPM (Phase 2)
		return true
	}

	return false
}
