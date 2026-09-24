package handler

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/rs/zerolog/log"
)

type MockStorage interface {
	UploadBinary(filepath string, data io.Reader, size int64) (string, error)
	UploadLog(filepath string, data io.Reader, size int64) (string, error)
	DownloadBlob(filepath string) (io.Reader, error)
}

type MockMinioStorage struct {
	client     *minio.Client
	bucketName string
}

func NewMinioStorage(endpoint, accessKey, secretKey, bucket string) (*MockMinioStorage, error) {
	minioClient, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: false,
	})
	if err != nil {
		log.Error().Err(err).Msg("failed to connect to minio")
		return nil, err
	}

	return &MockMinioStorage{
		client:     minioClient,
		bucketName: bucket, // use the parameter, not hardcoded
	}, nil
}

func (m MockMinioStorage) UploadBinary(filepath string, data io.Reader, size int64) (string, error) {
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

func (m MockMinioStorage) UploadLog(filepath string, data io.Reader, size int64) (string, error) {

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

func (m MockMinioStorage) DownloadBlob(objectName string) (io.Reader, error) {

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

func TestIsHealth(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	handler := &Handler{}
	handler.IsHealth(w, req)

	// 4. Check response
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	body := w.Body.String()
	if body != `{"status":"ok"}` {
		t.Errorf("expected {\"status\":\"ok\"}, got %s", body)
	}
}
