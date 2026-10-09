package storage

import "io"

// ============================================================
// Storage Interface
// ============================================================

type Storage interface {
	UploadArtifact(filepath string, data io.Reader, size int64) (string, error)
	UploadLog(testUUID, runID string, data io.Reader, size int64) (string, error)

	UploadBlob(objectName string, data io.Reader, size int64) (string, error)
	DownloadBlob(objectName string) (io.Reader, error)

	DeletePrefix(prefix string) error
	DeleteObject(objectName string) error
}
