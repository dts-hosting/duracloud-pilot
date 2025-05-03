package events

type SQSEvent struct {
	DetailType string `json:"detail-type"`
	Source     string `json:"source"`
	Detail     struct {
		Bucket struct {
			Name string `json:"name"`
		} `json:"bucket"`
		Object struct {
			Key  string `json:"key"`
			Size int    `json:"size,omitempty"`
			ETag string `json:"etag"`
		} `json:"object"`
		Reason string `json:"reason"`
	} `json:"detail"`
}

func (e *SQSEvent) BucketName() string {
	return e.Detail.Bucket.Name
}

func (e *SQSEvent) ObjectKey() string {
	return e.Detail.Object.Key
}
