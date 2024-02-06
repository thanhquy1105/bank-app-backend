// Package s3 implements media interface by storing media objects in Amazon S3 bucket.
package s3

import (
	"errors"
	"io"
	"net/http"
	"sync/atomic"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/thanhquy1105/simplebank/media"
	"github.com/thanhquy1105/simplebank/util"
)

const (
	handlerName = "s3"
	// Presign GET URLs for this number of seconds.
	presignDuration = 120
)

type awsconfig struct {
	AccessKeyId     string   `json:"access_key_id"`
	SecretAccessKey string   `json:"secret_access_key"`
	Region          string   `json:"region"`
	DisableSSL      bool     `json:"disable_ssl"`
	ForcePathStyle  bool     `json:"force_path_style"`
	Endpoint        string   `json:"endpoint"`
	BucketName      string   `json:"bucket"`
	CorsOrigins     []string `json:"cors_origins"`
}

type awshandler struct {
	svc  *s3.S3
	conf awsconfig
}

// readerCounter is a byte counter for bytes read through the io.Reader
type readerCounter struct {
	io.Reader
	count  int64
	reader io.Reader
}

// Read reads the bytes and records the number of read bytes.
func (rc *readerCounter) Read(buf []byte) (int, error) {
	n, err := rc.reader.Read(buf)
	atomic.AddInt64(&rc.count, int64(n))
	return n, err
}

// Init initializes the media handler.
func (ah *awshandler) Init(mediaConfig util.MediaConfig) error {
	var err error

	ah.conf = awsconfig{
		AccessKeyId:     mediaConfig.S3AccessKeyId,
		SecretAccessKey: mediaConfig.S3SecretAccessKey,
		Region:          mediaConfig.S3Region,
		DisableSSL:      mediaConfig.S3DisableSSL,
		ForcePathStyle:  mediaConfig.S3ForcePathStyle,
		Endpoint:        mediaConfig.S3EndPoint,
		BucketName:      mediaConfig.S3ButketName,
	}

	if ah.conf.AccessKeyId == "" {
		return errors.New("missing Access Key ID")
	}
	if ah.conf.SecretAccessKey == "" {
		return errors.New("missing Secret Access Key")
	}
	if ah.conf.Region == "" {
		return errors.New("missing Region")
	}
	if ah.conf.BucketName == "" {
		return errors.New("missing Bucket")
	}

	var sess *session.Session
	if sess, err = session.NewSession(&aws.Config{
		Region:           aws.String(ah.conf.Region),
		DisableSSL:       aws.Bool(ah.conf.DisableSSL),
		S3ForcePathStyle: aws.Bool(ah.conf.ForcePathStyle),
		Endpoint:         aws.String(ah.conf.Endpoint),
		Credentials:      credentials.NewStaticCredentials(ah.conf.AccessKeyId, ah.conf.SecretAccessKey, ""),
	}); err != nil {
		return err
	}

	// Create S3 service client
	ah.svc = s3.New(sess)

	// Check if bucket already exists.
	_, err = ah.svc.HeadBucket(&s3.HeadBucketInput{Bucket: aws.String(ah.conf.BucketName)})
	if err == nil {
		// Bucket exists
		return nil
	}

	if aerr, ok := err.(awserr.Error); !ok || aerr.Code() != "NotFound" {
		// Hard error.
		return err
	}

	// Bucket does not exist. Create one.
	_, err = ah.svc.CreateBucket(&s3.CreateBucketInput{Bucket: aws.String(ah.conf.BucketName)})
	if err != nil {
		// Check if someone has already created a bucket (possible in a cluster).
		if aerr, ok := err.(awserr.Error); ok {
			if aerr.Code() == s3.ErrCodeBucketAlreadyExists ||
				aerr.Code() == s3.ErrCodeBucketAlreadyOwnedByYou ||
				// Someone is already creating this bucket:
				// OperationAborted: A conflicting conditional operation is currently in progress against this resource.
				aerr.Code() == "OperationAborted" {
				// Clear benign error
				err = nil
			}
		}
	} else {
		// This is a new bucket.

		// The following serves two purposes:
		// 1. Setup CORS policy to be able to serve media directly from S3.
		// 2. Verify that the bucket is accessible to the current user.
		origins := ah.conf.CorsOrigins
		if len(origins) == 0 {
			origins = append(origins, "*")
		}
		_, err = ah.svc.PutBucketCors(&s3.PutBucketCorsInput{
			Bucket: aws.String(ah.conf.BucketName),
			CORSConfiguration: &s3.CORSConfiguration{
				CORSRules: []*s3.CORSRule{{
					AllowedMethods: aws.StringSlice([]string{http.MethodGet, http.MethodHead}),
					AllowedOrigins: aws.StringSlice(origins),
					AllowedHeaders: aws.StringSlice([]string{"*"}),
				}},
			},
		})
	}
	return err
}

// Upload processes request for a file upload. The file is given as io.Reader.
func (ah *awshandler) Upload(filename string, file io.ReadSeeker) (string, int64, error) {
	var err error

	uploader := s3manager.NewUploaderWithClient(ah.svc)

	rc := readerCounter{reader: file}
	result, err := uploader.Upload(&s3manager.UploadInput{
		Bucket: aws.String(ah.conf.BucketName),
		Key:    aws.String(filename),
		Body:   &rc,
	})

	if err != nil {
		return "", 0, err
	}

	return result.Location, rc.count, nil
}

// Delete deletes files from aws by provided slice of locations.
func (ah *awshandler) Delete(locations []string) error {
	toDelete := make([]s3manager.BatchDeleteObject, len(locations))
	for i, key := range locations {
		toDelete[i] = s3manager.BatchDeleteObject{
			Object: &s3.DeleteObjectInput{
				Key:    aws.String(key),
				Bucket: aws.String(ah.conf.BucketName),
			}}
	}
	batcher := s3manager.NewBatchDeleteWithClient(ah.svc)
	return batcher.Delete(aws.BackgroundContext(), &s3manager.DeleteObjectsIterator{
		Objects: toDelete,
	})
}

func init() {
	media.RegisterMediaHandler(handlerName, &awshandler{})
}
