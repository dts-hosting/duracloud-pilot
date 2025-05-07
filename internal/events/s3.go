package events

type S3EventRecord struct {
	EventName string `json:"eventName"`
	S3        struct {
		Bucket struct {
			Name string `json:"name"`
		} `json:"bucket"`
		Object struct {
			Key string `json:"key"`
		} `json:"object"`
	} `json:"s3"`
}

type S3Event struct {
	Records []S3EventRecord `json:"Records"`
}

func (e *S3Event) BucketName() string {
	if len(e.Records) > 0 {
		return e.Records[0].S3.Bucket.Name
	}
	return ""
}

func (e *S3Event) ObjectKey() string {
	if len(e.Records) > 0 {
		return e.Records[0].S3.Object.Key
	}
	return ""
}

func (e *S3Event) IsObjectCreatedEvent() bool {
	if len(e.Records) > 0 {
		return e.Records[0].EventName == "ObjectCreated:Put" ||
			e.Records[0].EventName == "ObjectCreated:Post" ||
			e.Records[0].EventName == "ObjectCreated:Copy" ||
			e.Records[0].EventName == "ObjectCreated:CompleteMultipartUpload"
	}
	return false
}

func (e *S3Event) IsObjectDeletedEvent() bool {
	if len(e.Records) > 0 {
		return e.Records[0].EventName == "ObjectRemoved:Delete" ||
			e.Records[0].EventName == "ObjectRemoved:DeleteMarkerCreated"
	}
	return false
}
