package helpers

import "fmt"
import "regexp"
import "strings"

const (
	IsBucketRequestedSuffix = "-bucket-requested"
	IsDuraCloudPrefix       = "duracloud-"
	IsLogsSuffix            = "-logs"
	IsManagedSuffix         = "-managed"
	IsPublicSuffix          = "-public"
	IsReplicationSuffix     = "-replication"
)

func IsBucketRequestBucket(name string) bool {
	return strings.HasSuffix(name, IsBucketRequestedSuffix)
}

// IsIgnoreFilesBucket buckets excluded from checksum processing
func IsIgnoreFilesBucket(name string) bool {
	return IsBucketRequestBucket(name) || IsRestrictedBucket(name)
}

func IsDuraCloudBucket(name string) bool {
	return strings.HasPrefix(name, IsDuraCloudPrefix)
}

func IsLogsBucket(name string) bool {
	return strings.HasSuffix(name, IsLogsSuffix)
}

func IsManagedBucket(name string) bool {
	return strings.HasSuffix(name, IsManagedSuffix)
}

func IsPublicBucket(name string) bool {
	return strings.HasSuffix(name, IsPublicSuffix)
}

func IsReplicationBucket(name string) bool {
	return strings.HasSuffix(name, IsReplicationSuffix)
}

// IsRestrictedBucket buckets with restricted access permissions for s3 users
func IsRestrictedBucket(name string) bool {
	return IsLogsBucket(name) || IsManagedBucket(name) || IsReplicationBucket(name)
}

func ValidateBucketName(name string) bool {
	var (
		whitelist  = "a-zA-Z0-9-"
		disallowed = regexp.MustCompile(fmt.Sprintf("[^%s]+", whitelist))
	)
	return !disallowed.MatchString(name)
}


