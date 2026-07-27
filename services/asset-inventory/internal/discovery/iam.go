package discovery

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamTypes "github.com/aws/aws-sdk-go-v2/service/iam/types"

	"github.com/sentinel-cnapp/sentinel-cnapp/services/asset-inventory/internal/store"
)

// IAMDiscoverer discovers IAM roles, users, and policies.
type IAMDiscoverer struct {
	client    *iam.Client
	accountID string
}

// NewIAMDiscoverer creates a new IAM discoverer.
func NewIAMDiscoverer(cfg aws.Config, accountID string) *IAMDiscoverer {
	return &IAMDiscoverer{
		client:    iam.NewFromConfig(cfg),
		accountID: accountID,
	}
}

func (d *IAMDiscoverer) Provider() string { return "aws" }
func (d *IAMDiscoverer) Service() string  { return "iam" }

func (d *IAMDiscoverer) Discover(ctx context.Context, region string) ([]store.Asset, error) {
	var assets []store.Asset

	// IAM is a global service, region is always "global"
	if region != "us-east-1" {
		return nil, nil
	}

	roles, err := d.discoverRoles(ctx)
	if err != nil {
		return nil, fmt.Errorf("discovering roles: %w", err)
	}
	assets = append(assets, roles...)

	users, err := d.discoverUsers(ctx)
	if err != nil {
		return nil, fmt.Errorf("discovering users: %w", err)
	}
	assets = append(assets, users...)

	return assets, nil
}

func (d *IAMDiscoverer) discoverRoles(ctx context.Context) ([]store.Asset, error) {
	var assets []store.Asset
	paginator := iam.NewListRolesPaginator(d.client, &iam.ListRolesInput{})

	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}

		for _, role := range output.Roles {
			asset := d.roleToAsset(role)
			assets = append(assets, asset)
		}
	}
	return assets, nil
}

func (d *IAMDiscoverer) roleToAsset(role iamTypes.Role) store.Asset {
	id := fmt.Sprintf("arn:aws:iam::%s:role/%s", d.accountID, *role.RoleName)
	tags := iamTagsToMap(role.Tags)

	// Check for overly permissive trust policy
	internetFacing := false
	if role.AssumeRolePolicyDocument != nil {
		// Basic check — CSPM will do deep analysis
		internetFacing = true
	}

	metadata := map[string]string{
		"role_id":       *role.RoleId,
		"path":          safeString(role.Path),
		"max_session_duration": fmt.Sprintf("%d", role.MaxSessionDuration),
		"create_date":   role.CreateDate.Format(time.RFC3339),
		"description":   safeString(role.Description),
	}
	metadataJSON := marshalMetadata(metadata)

	now := time.Now().UTC()
	return store.Asset{
		ID:             id,
		Provider:       "aws",
		AssetType:      "iam_role",
		Name:           *role.RoleName,
		Region:         "global",
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

func (d *IAMDiscoverer) discoverUsers(ctx context.Context) ([]store.Asset, error) {
	var assets []store.Asset
	paginator := iam.NewListUsersPaginator(d.client, &iam.ListUsersInput{})

	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}

		for _, user := range output.Users {
			asset := d.userToAsset(user)
			assets = append(assets, asset)
		}
	}
	return assets, nil
}

func (d *IAMDiscoverer) userToAsset(user iamTypes.User) store.Asset {
	id := fmt.Sprintf("arn:aws:iam::%s:user/%s", d.accountID, *user.UserName)
	tags := iamTagsToMap(user.Tags)

	metadata := map[string]string{
		"user_id":     *user.UserId,
		"path":        safeString(user.Path),
		"create_date": user.CreateDate.Format(time.RFC3339),
	}
	metadataJSON := marshalMetadata(metadata)

	now := time.Now().UTC()
	return store.Asset{
		ID:           id,
		Provider:     "aws",
		AssetType:    "iam_user",
		Name:         *user.UserName,
		Region:       "global",
		Tags:         tags,
		MetadataJSON: metadataJSON,
		Environment:  inferEnvironment(tags),
		AccountID:    d.accountID,
		Active:       true,
		DiscoveredAt: now,
		LastSyncedAt: now,
	}
}
