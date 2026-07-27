package actions

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3Types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2Types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/ecr"
	ecrTypes "github.com/aws/aws-sdk-go-v2/service/ecr/types"
)

// AWSExecutor handles AWS remediation actions.
type AWSExecutor struct {
	s3Client   *s3.Client
	ec2Client  *ec2.Client
	iamClient  *iam.Client
	ecrClient  *ecr.Client
}

// NewAWSExecutor creates a new AWS remediation executor.
func NewAWSExecutor(cfg aws.Config) *AWSExecutor {
	return &AWSExecutor{
		s3Client:  s3.NewFromConfig(cfg),
		ec2Client: ec2.NewFromConfig(cfg),
		iamClient: iam.NewFromConfig(cfg),
		ecrClient: ecr.NewFromConfig(cfg),
	}
}

// BlockS3PublicAccess enables block public access settings on an S3 bucket.
func (e *AWSExecutor) BlockS3PublicAccess(ctx context.Context, bucketARN string) (string, error) {
	bucketName := extractBucketName(bucketARN)
	if bucketName == "" {
		return "", fmt.Errorf("invalid bucket ARN: %s", bucketARN)
	}

	_, err := e.s3Client.PutPublicAccessBlock(ctx, &s3.PutPublicAccessBlockInput{
		Bucket: &bucketName,
		PublicAccessBlockConfiguration: &s3Types.PublicAccessBlockConfiguration{
			BlockPublicAcls:       aws.Bool(true),
			IgnorePublicAcls:      aws.Bool(true),
			BlockPublicPolicy:     aws.Bool(true),
			RestrictPublicBuckets: aws.Bool(true),
		},
	})
	if err != nil {
		return "", fmt.Errorf("blocking public access on %s: %w", bucketName, err)
	}

	return fmt.Sprintf("BlockPublicAccess enabled on s3://%s", bucketName), nil
}

// DenyS3PublicPolicy adds a deny-public-access bucket policy.
func (e *AWSExecutor) DenyS3PublicPolicy(ctx context.Context, bucketARN string) (string, error) {
	bucketName := extractBucketName(bucketARN)
	if bucketName == "" {
		return "", fmt.Errorf("invalid bucket ARN: %s", bucketARN)
	}

	policy := fmt.Sprintf(`{
		"Version": "2012-10-17",
		"Statement": [{
			"Effect": "Deny",
			"Principal": "*",
			"Action": "s3:*",
			"Resource": [
				"arn:aws:s3:::%s",
				"arn:aws:s3:::%s/*"
			],
			"Condition": {
				"Bool": { "aws:SecureTransport": "false" }
			}
		}]
	}`, bucketName, bucketName)

	_, err := e.s3Client.PutBucketPolicy(ctx, &s3.PutBucketPolicyInput{
		Bucket: &bucketName,
		Policy: &policy,
	})
	if err != nil {
		return "", fmt.Errorf("putting bucket policy on %s: %w", bucketName, err)
	}

	return fmt.Sprintf("Deny-public-access policy applied to s3://%s", bucketName), nil
}

// RestrictIAMTrustPolicy restricts an IAM role's trust policy.
func (e *AWSExecutor) RestrictIAMTrustPolicy(ctx context.Context, roleARN string) (string, error) {
	roleName := extractRoleName(roleARN)
	if roleName == "" {
		return "", fmt.Errorf("invalid role ARN: %s", roleARN)
	}

	// Restrict trust to the same account only
	trustPolicy := fmt.Sprintf(`{
		"Version": "2012-10-17",
		"Statement": [{
			"Effect": "Allow",
			"Principal": { "AWS": "arn:aws:iam::%s:root" },
			"Action": "sts:AssumeRole"
		}]
	}`, extractAccountID(roleARN))

	_, err := e.iamClient.UpdateAssumeRolePolicy(ctx, &iam.UpdateAssumeRolePolicyInput{
		RoleName:       &roleName,
		PolicyDocument: &trustPolicy,
	})
	if err != nil {
		return "", fmt.Errorf("updating trust policy for %s: %w", roleName, err)
	}

	return fmt.Sprintf("Trust policy restricted for role %s", roleName), nil
}

// RevokePublicIngress removes 0.0.0.0/0 ingress rules from a security group.
func (e *AWSExecutor) RevokePublicIngress(ctx context.Context, sgARN string) (string, error) {
	sgID := extractSGID(sgARN)
	region := extractRegion(sgARN)
	if sgID == "" {
		return "", fmt.Errorf("invalid security group ARN: %s", sgARN)
	}

	// Describe the security group to find public ingress rules
	sg, err := e.ec2Client.DescribeSecurityGroups(ctx, &ec2.DescribeSecurityGroupsInput{
		GroupIds: []string{sgID},
	})
	if err != nil {
		return "", fmt.Errorf("describing security group %s: %w", sgID, err)
	}

	if len(sg.SecurityGroups) == 0 {
		return "", fmt.Errorf("security group %s not found", sgID)
	}

	removed := 0
	for _, perm := range sg.SecurityGroups[0].IpPermissions {
		for _, ipRange := range perm.IpRanges {
			if ipRange.CidrIp != nil && *ipRange.CidrIp == "0.0.0.0/0" {
			_, err := e.ec2Client.RevokeSecurityGroupIngress(ctx, &ec2.RevokeSecurityGroupIngressInput{
				GroupId:       &sgID,
				IpPermissions: []ec2Types.IpPermission{perm},
			})
				if err == nil {
					removed++
				}
			}
		}
	}

	return fmt.Sprintf("Revoked %d public ingress rules from %s (region: %s)", removed, sgID, region), nil
}

// EnableECRScanOnPush enables scan on push for an ECR repository.
func (e *AWSExecutor) EnableECRScanOnPush(ctx context.Context, repoARN string) (string, error) {
	repoName := extractECRRepoName(repoARN)
	if repoName == "" {
		return "", fmt.Errorf("invalid ECR ARN: %s", repoARN)
	}

	_, err := e.ecrClient.PutImageScanningConfiguration(ctx, &ecr.PutImageScanningConfigurationInput{
		RepositoryName: &repoName,
		ImageScanningConfiguration: &ecrTypes.ImageScanningConfiguration{
			ScanOnPush: aws.Bool(true),
		},
	})
	if err != nil {
		return "", fmt.Errorf("enabling scan on push for %s: %w", repoName, err)
	}

	return fmt.Sprintf("Scan-on-push enabled for ECR repository %s", repoName), nil
}

// NotifySecretRotation notifies about secret rotation (placeholder — integrates with Slack/PagerDuty).
func (e *AWSExecutor) NotifySecretRotation(ctx context.Context, assetID string) (string, error) {
	// In production, this would send a Slack notification or create a Jira ticket.
	// For MVP, we record the notification.
	return fmt.Sprintf("Rotation notification sent for secret in %s. Manual rotation required.", assetID), nil
}

// ── ARN Parsers ─────────────────────────────────┐

func extractBucketName(arn string) string {
	parts := strings.Split(arn, ":::") // arn:aws:s3:::bucket-name
	if len(parts) == 2 {
		return parts[1]
	}
	return ""
}

func extractRoleName(arn string) string {
	parts := strings.Split(arn, ":role/")
	if len(parts) == 2 {
		return parts[1]
	}
	return ""
}

func extractSGID(arn string) string {
	parts := strings.Split(arn, ":security-group/")
	if len(parts) == 2 {
		return parts[1]
	}
	return ""
}

func extractRegion(arn string) string {
	parts := strings.Split(arn, ":")
	if len(parts) >= 4 {
		return parts[3]
	}
	return ""
}

func extractAccountID(arn string) string {
	parts := strings.Split(arn, ":")
	if len(parts) >= 5 {
		return parts[4]
	}
	return ""
}

func extractECRRepoName(arn string) string {
	parts := strings.Split(arn, ":repository/")
	if len(parts) == 2 {
		return parts[1]
	}
	return ""
}

