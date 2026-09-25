package storage

import (
	"context"
	"io"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/rs/zerolog/log"
)

type Storage interface {
	UploadArtifact(filepath string, data io.Reader, size int64) (string, error)
	UploadLog(filepath string, data io.Reader, size int64) (string, error)
	DownloadBlob(filepath string) (io.Reader, error)
	DeletePrefix(prefix string) error
	DeleteObject(objectName string) error
}

type MinioStorage struct {
	client     *minio.Client
	bucketName string
}

func NewMinioStorage(endpoint, accessKey, secretKey, bucket string) (*MinioStorage, error) {
	minioClient, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: false,
	})
	if err != nil {
		log.Error().Err(err).Msg("failed to connect to minio")
		return nil, err
	}

	return &MinioStorage{
		client:     minioClient,
		bucketName: bucket, // use the parameter, not hardcoded
	}, nil
}

func (m MinioStorage) UploadArtifact(filepath string, data io.Reader, size int64) (string, error) {
	objectName := filepath + "/artifact"

	_, err := m.client.PutObject(
		context.Background(),
		m.bucketName,
		objectName,
		data,
		int64(size),
		minio.PutObjectOptions{},
	)
	if err != nil {
		return "", err
	}

	return objectName, nil
}

func (m MinioStorage) UploadLog(filepath string, data io.Reader, size int64) (string, error) {

	objectName := filepath + "/logs/" + time.Now().UTC().Format(time.RFC3339) + ".txt"

	_, err := m.client.PutObject(
		context.Background(),
		m.bucketName,
		objectName,
		data,
		int64(size),
		minio.PutObjectOptions{},
	)
	if err != nil {
		return "", err
	}

	return objectName, nil
}

func (m MinioStorage) DownloadBlob(objectName string) (io.Reader, error) {

	object, err := m.client.GetObject(
		context.Background(),
		m.bucketName,
		objectName,
		minio.GetObjectOptions{},
	)
	if err != nil {
		return nil, err
	}

	return object, nil
}

func (m MinioStorage) DeletePrefix(prefix string) error {
	ctx := context.Background()
	for object := range m.client.ListObjects(ctx, m.bucketName, minio.ListObjectsOptions{
		Prefix:    prefix,
		Recursive: true,
	}) {
		if object.Err != nil {
			return object.Err
		}
		if err := m.client.RemoveObject(ctx, m.bucketName, object.Key, minio.RemoveObjectOptions{}); err != nil {
			return err
		}
	}
	return nil
}

func (m MinioStorage) DeleteObject(objectName string) error {
	if objectName == "" {
		return nil
	}
	ctx := context.Background()
	return m.client.RemoveObject(ctx, m.bucketName, objectName, minio.RemoveObjectOptions{})
}
