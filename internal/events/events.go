package events

import "strings"

// BucketNamer defines an interface for types that have a BucketName() method
type BucketNamer interface {
	BucketName() string
}

func IsEventLogsBucket(b BucketNamer) bool {
	return strings.Contains(b.BucketName(), "-event-logs")
}

func IsManagedBucket(b BucketNamer) bool {
	return strings.Contains(b.BucketName(), "-managed")
}

func IsPublicBucket(b BucketNamer) bool {
	return strings.Contains(b.BucketName(), "-public")
}

func IsReplicationBucket(b BucketNamer) bool {
	return strings.Contains(b.BucketName(), "-replication")
}

func IsRestrictedBucket(b BucketNamer) bool {
	return IsEventLogsBucket(b) || IsManagedBucket(b) || IsReplicationBucket(b)
}
