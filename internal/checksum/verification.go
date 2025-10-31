package checksum

import (
	"context"
	"duracloud/internal/db"
	"duracloud/internal/files"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type Verifier struct {
	ctx      context.Context
	db       *db.DB
	s3Client *s3.Client
	obj      files.S3Object
}

func NewVerifier(ctx context.Context, db *db.DB, s3Client *s3.Client, obj files.S3Object) *Verifier {
	return &Verifier{
		ctx:      ctx,
		db:       db,
		s3Client: s3Client,
		obj:      obj,
	}
}

func (v *Verifier) Deposit(etag string) error {
	nextScheduledTime, err := db.GetNextScheduledTime()
	if err != nil {
		return err
	}

	calc := NewS3Calculator(v.s3Client)
	hash, err := calc.CalculateChecksum(v.ctx, v.obj)

	// Optimistic outlook for our adventurer checksum record
	checksumRecord := db.ChecksumRecord{
		BucketName:          v.obj.Bucket,
		ObjectKey:           v.obj.Key,
		Checksum:            hash, // May be empty if failed
		LastChecksumDate:    time.Now(),
		LastChecksumMessage: "ok",
		LastChecksumSuccess: true,
		NextChecksumDate:    nextScheduledTime,
	}

	if err != nil {
		// Checksum calculation failed
		checksumRecord.LastChecksumMessage = err.Error()
		checksumRecord.LastChecksumSuccess = false
	} else if !strings.Contains(etag, "-") && hash != etag {
		// ETag validation failed
		msg := fmt.Sprintf("checksum does not match etag: calculated=%s etag=%s", hash, etag)
		log.Println(msg)
		checksumRecord.LastChecksumMessage = msg
		checksumRecord.LastChecksumSuccess = false
	}

	err = v.db.Put(checksumRecord)
	if err != nil {
		return err
	}

	if checksumRecord.LastChecksumSuccess {
		err = v.db.Schedule(checksumRecord)
		if err != nil {
			return err
		}
	}

	return nil
}

func (v *Verifier) Verify() (bool, error) {
	log.Printf("Starting checksum verification for: %s/%s", v.obj.Bucket, v.obj.Key)
	ok := true

	currentTime := time.Now()
	nextScheduledTime, err := db.GetNextScheduledTime()
	if err != nil {
		return false, err
	}

	checksumRecord, err := v.db.Get(v.obj)
	if err != nil {
		return false, err
	}

	checksumRecord.LastChecksumDate = currentTime
	checksumRecord.NextChecksumDate = nextScheduledTime

	calc := NewS3Calculator(v.s3Client)
	checksumResult, err := calc.CalculateChecksum(v.ctx, v.obj)
	if err != nil {
		checksumRecord.LastChecksumMessage = err.Error()
		checksumRecord.LastChecksumSuccess = false
		ok = false
	} else if checksumResult != checksumRecord.Checksum {
		msg := fmt.Sprintf("Checksum mismatch: calculated=%s, stored=%s", checksumResult, checksumRecord.Checksum)
		log.Println(msg)
		checksumRecord.LastChecksumMessage = msg
		checksumRecord.LastChecksumSuccess = false
		ok = false
	} else {
		// Technically this is redundant but included for clarity
		checksumRecord.LastChecksumMessage = "ok"
		checksumRecord.LastChecksumSuccess = true
	}

	err = v.db.Put(checksumRecord)
	if err != nil {
		log.Printf("Failed to update checksum record due to : %v", err)
		return ok, err
	}

	if checksumRecord.LastChecksumSuccess {
		log.Printf("Checksum verification succeeded for: %s/%s", v.obj.Bucket, v.obj.Key)
		err = v.db.Schedule(checksumRecord)
		if err != nil {
			return ok, err
		}
	}

	return ok, nil
}
