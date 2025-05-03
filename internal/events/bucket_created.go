package events

type BucketCreatedEvent struct {
	Detail struct {
		RequestParameters struct {
			BucketName string `json:"bucketName"`
		} `json:"requestParameters"`
	} `json:"detail"`
}

func (e *BucketCreatedEvent) BucketName() string {
	return e.Detail.RequestParameters.BucketName
}
