package storage

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// ============================================================
// MinIO Storage
// ============================================================

type MinioStorage struct {
	client     *minio.Client
	bucketName string
}

// NewMinioStorage creates a MinIO storage client.
//
// endpoint examples:
//
//	localhost:9000
//	minio:9000
//	https://minio.example.com
func NewMinioStorage(
	endpoint string,
	accessKey string,
	secretKey string,
	bucketName string,
	region string,
) (*MinioStorage, error) {

	if endpoint == "" {
		return nil, fmt.Errorf("minio endpoint is required")
	}

	if accessKey == "" {
		return nil, fmt.Errorf("minio access key is required")
	}

	if secretKey == "" {
		return nil, fmt.Errorf("minio secret key is required")
	}

	if bucketName == "" {
		return nil, fmt.Errorf("minio bucket is required")
	}

	endpoint = strings.TrimSpace(endpoint)

	useSSL := false

	if strings.HasPrefix(endpoint, "https://") {
		useSSL = true
		endpoint = strings.TrimPrefix(endpoint, "https://")
	} else {
		endpoint = strings.TrimPrefix(endpoint, "http://")
	}

	client, err := minio.New(endpoint, &minio.Options{
		Creds: credentials.NewStaticV4(
			accessKey,
			secretKey,
			"",
		),
		Secure: useSSL,
		Region: region,
	})
	if err != nil {
		return nil, fmt.Errorf("create minio client: %w", err)
	}

	return &MinioStorage{
		client:     client,
		bucketName: bucketName,
	}, nil
}

// UploadArtifact uploads a test artifact.
//
// Object:
//
//	<filepath>/artifact
func (s *MinioStorage) UploadArtifact(
	filepath string,
	data io.Reader,
	size int64,
) (string, error) {

	objectName := filepath + "/artifact"

	_, err := s.client.PutObject(
		context.Background(),
		s.bucketName,
		objectName,
		data,
		size,
		minio.PutObjectOptions{
			ContentType: "application/octet-stream",
		},
	)

	if err != nil {
		return "", fmt.Errorf("minio upload artifact: %w", err)
	}

	return objectName, nil
}

// UploadLog uploads a test execution log.
//
// Object:
//
//	<testUUID>/log/<runID>/log
func (s *MinioStorage) UploadLog(
	testUUID string,
	runID string,
	data io.Reader,
	size int64,
) (string, error) {

	objectName := testUUID + "/log/" + runID + "/log"

	_, err := s.client.PutObject(
		context.Background(),
		s.bucketName,
		objectName,
		data,
		size,
		minio.PutObjectOptions{
			ContentType: "text/plain",
		},
	)

	if err != nil {
		return "", fmt.Errorf("minio upload log: %w", err)
	}

	return objectName, nil
}

// UploadBlob uploads an arbitrary object.
func (s *MinioStorage) UploadBlob(
	objectName string,
	data io.Reader,
	size int64,
) (string, error) {

	_, err := s.client.PutObject(
		context.Background(),
		s.bucketName,
		objectName,
		data,
		size,
		minio.PutObjectOptions{
			ContentType: "application/octet-stream",
		},
	)

	if err != nil {
		return "", fmt.Errorf("minio upload blob: %w", err)
	}

	return objectName, nil
}

// DownloadBlob downloads an object.
//
// IMPORTANT:
// The returned io.Reader is backed by an open MinIO object.
// The caller should close it if it supports io.Closer.
func (s *MinioStorage) DownloadBlob(
	objectName string,
) (io.Reader, error) {

	object, err := s.client.GetObject(
		context.Background(),
		s.bucketName,
		objectName,
		minio.GetObjectOptions{},
	)
	if err != nil {
		return nil, fmt.Errorf("minio get object: %w", err)
	}

	// GetObject is lazy. Stat() forces MinIO to verify that
	// the object actually exists.
	_, err = object.Stat()
	if err != nil {
		object.Close()
		return nil, fmt.Errorf("minio object does not exist: %w", err)
	}

	return object, nil
}

// DeletePrefix deletes every object whose name starts with prefix.
func (s *MinioStorage) DeletePrefix(prefix string) error {

	objects := s.client.ListObjects(
		context.Background(),
		s.bucketName,
		minio.ListObjectsOptions{
			Prefix:    prefix,
			Recursive: true,
		},
	)

	for object := range objects {
		if object.Err != nil {
			return fmt.Errorf(
				"minio list objects for prefix %q: %w",
				prefix,
				object.Err,
			)
		}

		err := s.client.RemoveObject(
			context.Background(),
			s.bucketName,
			object.Key,
			minio.RemoveObjectOptions{},
		)
		if err != nil {
			return fmt.Errorf(
				"minio delete object %q: %w",
				object.Key,
				err,
			)
		}
	}

	return nil
}

// DeleteObject deletes one object.
func (s *MinioStorage) DeleteObject(
	objectName string,
) error {

	err := s.client.RemoveObject(
		context.Background(),
		s.bucketName,
		objectName,
		minio.RemoveObjectOptions{},
	)

	if err != nil {
		return fmt.Errorf("minio delete object: %w", err)
	}

	return nil
}
