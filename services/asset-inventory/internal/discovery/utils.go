package discovery

import (
	"encoding/json"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	iamTypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	s3Types "github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// safeString returns an empty string if the pointer is nil.
func safeString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// ec2TagsToMap converts EC2 tags to a map.
func ec2TagsToMap(tags []types.Tag) map[string]string {
	m := make(map[string]string)
	for _, t := range tags {
		if t.Key == nil {
			continue
		}
		val := ""
		if t.Value != nil {
			val = *t.Value
		}
		m[*t.Key] = val
	}
	return m
}

// iamTagsToMap converts IAM tags to a map.
func iamTagsToMap(tags []iamTypes.Tag) map[string]string {
	m := make(map[string]string)
	for _, t := range tags {
		m[*t.Key] = safeString(t.Value)
	}
	return m
}

// awsTagsToMap converts a generic AWS SDK tag map to our standard format.
func awsTagsToMap(tags map[string]string) map[string]string {
	if tags == nil {
		return make(map[string]string)
	}
	return tags
}

// s3TagsToMap converts S3 tags to a map.
func s3TagsToMap(tags []s3Types.Tag) map[string]string {
	m := make(map[string]string)
	for _, t := range tags {
		m[*t.Key] = *t.Value
	}
	return m
}

// rdsTagsToMap converts RDS tags to a map.
func rdsTagsToMap(tags []iamTypes.Tag) map[string]string {
	m := make(map[string]string)
	for _, t := range tags {
		m[*t.Key] = safeString(t.Value)
	}
	return m
}

// marshalMetadata marshals a metadata map to JSON string.
func marshalMetadata(metadata map[string]string) string {
	data, err := json.Marshal(metadata)
	if err != nil {
		return "{}"
	}
	return string(data)
}

// inferEnvironment tries to determine the environment from tags.
func inferEnvironment(tags map[string]string) string {
	for _, key := range []string{"Environment", "environment", "env", "Env"} {
		if val, ok := tags[key]; ok {
			lower := strings.ToLower(val)
			switch lower {
			case "prod", "production":
				return "production"
			case "staging", "stage":
				return "staging"
			case "dev", "development":
				return "development"
			case "test", "testing":
				return "test"
			}
			return lower
		}
	}
	return "production" // default
}
