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
	UploadBinary(filepath string, data io.Reader, size int64) (string, error)
	UploadLog(filepath string, data io.Reader, size int64) (string, error)
	DownloadBlob(filepath string) (io.Reader, error)
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

func (m MinioStorage) UploadBinary(filepath string, data io.Reader, size int64) (string, error) {
	objectName := filepath + "/binary"

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

	objectName := filepath + "/logs/" + time.Now().Format(time.RFC3339) + ".txt"

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
