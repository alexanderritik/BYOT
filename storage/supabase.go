package storage

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// ============================================================
// Supabase Storage
// ============================================================

type SupabaseStorage struct {
	baseURL    string
	key        string
	bucketName string

	httpClient *http.Client
}

// NewSupabaseStorage creates a Supabase Storage REST API client.
//
// baseURL:
//
//	https://<project-ref>.supabase.co
//
// key:
//
//	Supabase server-side secret key.
//	Prefer the new sb_secret_... key.
//
// bucket:
//
//	Your Storage bucket, e.g. BYOT
func NewSupabaseStorage(
	baseURL string,
	key string,
	bucketName string,
) (*SupabaseStorage, error) {

	if baseURL == "" {
		return nil, fmt.Errorf("supabase URL is required")
	}

	if key == "" {
		return nil, fmt.Errorf("supabase key is required")
	}

	if bucketName == "" {
		return nil, fmt.Errorf("supabase bucket is required")
	}

	baseURL = strings.TrimSpace(baseURL)
	baseURL = strings.TrimRight(baseURL, "/")

	// Allow users to accidentally provide:
	//
	// https://project.supabase.co/storage/v1
	//
	// but normalize it back to the project URL.
	baseURL = strings.TrimSuffix(
		baseURL,
		"/storage/v1",
	)

	return &SupabaseStorage{
		baseURL:    baseURL,
		key:        key,
		bucketName: bucketName,

		httpClient: &http.Client{
			Timeout: 10 * time.Minute,
		},
	}, nil
}

// ============================================================
// Supabase URL Helpers
// ============================================================

func (s *SupabaseStorage) objectURL(
	objectName string,
) string {

	return s.baseURL +
		"/storage/v1/object/" +
		url.PathEscape(s.bucketName) +
		"/" +
		objectName
}

func (s *SupabaseStorage) listURL() string {

	return s.baseURL +
		"/storage/v1/object/list/" +
		url.PathEscape(s.bucketName)
}

func (s *SupabaseStorage) deleteURL() string {

	return s.baseURL +
		"/storage/v1/object/" +
		url.PathEscape(s.bucketName)
}

// ============================================================
// Supabase Headers
// ============================================================

func (s *SupabaseStorage) setHeaders(
	req *http.Request,
) {

	// IMPORTANT:
	//
	// Do NOT use:
	//
	// Authorization: Bearer <sb_secret_...>
	//
	// New Supabase secret keys are not JWTs.
	//
	// Use apikey instead.
	req.Header.Set("apikey", s.key)
}

// ============================================================
// Supabase Upload
// ============================================================

// UploadArtifact uploads:
//
//	<filepath>/artifact
func (s *SupabaseStorage) UploadArtifact(
	filepath string,
	data io.Reader,
	size int64,
) (string, error) {

	objectName := filepath + "/artifact"

	err := s.upload(
		objectName,
		data,
		size,
		"application/octet-stream",
	)
	if err != nil {
		return "", fmt.Errorf("supabase upload artifact: %w", err)
	}

	return objectName, nil
}

// UploadLog uploads:
//
//	<testUUID>/log/<runID>/log
func (s *SupabaseStorage) UploadLog(
	testUUID string,
	runID string,
	data io.Reader,
	size int64,
) (string, error) {

	objectName := testUUID + "/log/" + runID + "/log"

	err := s.upload(
		objectName,
		data,
		size,
		"text/plain",
	)
	if err != nil {
		return "", fmt.Errorf("supabase upload log: %w", err)
	}

	return objectName, nil
}

// UploadBlob uploads an arbitrary object.
func (s *SupabaseStorage) UploadBlob(
	objectName string,
	data io.Reader,
	size int64,
) (string, error) {

	err := s.upload(
		objectName,
		data,
		size,
		"application/octet-stream",
	)
	if err != nil {
		return "", fmt.Errorf("supabase upload blob: %w", err)
	}

	return objectName, nil
}

// upload performs the actual Supabase REST upload.
func (s *SupabaseStorage) upload(
	objectName string,
	data io.Reader,
	size int64,
	contentType string,
) error {

	req, err := http.NewRequest(
		http.MethodPost,
		s.objectURL(objectName),
		data,
	)
	if err != nil {
		return fmt.Errorf("create upload request: %w", err)
	}

	s.setHeaders(req)

	req.Header.Set(
		"Content-Type",
		contentType,
	)

	req.Header.Set(
		"x-upsert",
		"true",
	)

	req.ContentLength = size

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf(
			"upload request failed: %w",
			err,
		)
	}

	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {

		body, _ := io.ReadAll(
			io.LimitReader(resp.Body, 4096),
		)

		return fmt.Errorf(
			"upload failed: status=%d body=%s",
			resp.StatusCode,
			string(body),
		)
	}

	return nil
}

// ============================================================
// Supabase Download
// ============================================================

func (s *SupabaseStorage) DownloadBlob(
	objectName string,
) (io.Reader, error) {

	req, err := http.NewRequest(
		http.MethodGet,
		s.objectURL(objectName),
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"create download request: %w",
			err,
		)
	}

	s.setHeaders(req)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf(
			"download request failed: %w",
			err,
		)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {

		body, _ := io.ReadAll(
			io.LimitReader(resp.Body, 4096),
		)

		resp.Body.Close()

		return nil, fmt.Errorf(
			"download failed: status=%d body=%s",
			resp.StatusCode,
			string(body),
		)
	}

	// IMPORTANT:
	//
	// Do not close resp.Body here.
	//
	// The caller receives the reader and must consume/close it.
	return resp.Body, nil
}

// ============================================================
// Supabase List Objects
// ============================================================

type supabaseListRequest struct {
	Prefix string `json:"prefix"`

	Limit int `json:"limit"`

	Offset int `json:"offset"`

	SortBy supabaseSortBy `json:"sortBy"`
}

type supabaseSortBy struct {
	Column string `json:"column"`

	Order string `json:"order"`
}

type supabaseObject struct {
	Name string `json:"name"`
}

// listObjects returns object names under a prefix.
//
// This method paginates through the entire prefix.
func (s *SupabaseStorage) listObjects(
	prefix string,
) ([]string, error) {

	const pageSize = 1000

	var objects []string

	offset := 0

	for {

		payload := supabaseListRequest{
			Prefix: prefix,
			Limit:  pageSize,
			Offset: offset,
			SortBy: supabaseSortBy{
				Column: "name",
				Order:  "asc",
			},
		}

		body, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf(
				"marshal list request: %w",
				err,
			)
		}

		req, err := http.NewRequest(
			http.MethodPost,
			s.listURL(),
			bytes.NewReader(body),
		)
		if err != nil {
			return nil, fmt.Errorf(
				"create list request: %w",
				err,
			)
		}

		s.setHeaders(req)

		req.Header.Set(
			"Content-Type",
			"application/json",
		)

		resp, err := s.httpClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf(
				"list request failed: %w",
				err,
			)
		}

		responseBody, readErr := io.ReadAll(
			io.LimitReader(resp.Body, 10*1024*1024),
		)

		resp.Body.Close()

		if readErr != nil {
			return nil, fmt.Errorf(
				"read list response: %w",
				readErr,
			)
		}

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil, fmt.Errorf(
				"list failed: status=%d body=%s",
				resp.StatusCode,
				string(responseBody),
			)
		}

		var page []supabaseObject

		if err := json.Unmarshal(
			responseBody,
			&page,
		); err != nil {
			return nil, fmt.Errorf(
				"decode list response: %w",
				err,
			)
		}

		for _, object := range page {

			name := object.Name

			// Supabase may return folder-like entries.
			// If the API returns a folder instead of an actual
			// object, skip it.
			if name == "" {
				continue
			}

			objects = append(
				objects,
				name,
			)
		}

		if len(page) < pageSize {
			break
		}

		offset += pageSize
	}

	return objects, nil
}

// ============================================================
// Supabase Delete
// ============================================================

func (s *SupabaseStorage) DeletePrefix(
	prefix string,
) error {

	objects, err := s.listObjects(prefix)
	if err != nil {
		return fmt.Errorf(
			"list objects for prefix %q: %w",
			prefix,
			err,
		)
	}

	if len(objects) == 0 {
		return nil
	}

	// Supabase bulk delete accepts a list of object paths.
	//
	// Keep batches reasonably sized.
	const batchSize = 1000

	for start := 0; start < len(objects); start += batchSize {

		end := start + batchSize

		if end > len(objects) {
			end = len(objects)
		}

		batch := objects[start:end]

		payload := struct {
			Prefixes []string `json:"prefixes"`
		}{
			Prefixes: batch,
		}

		body, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf(
				"marshal delete request: %w",
				err,
			)
		}

		req, err := http.NewRequest(
			http.MethodDelete,
			s.deleteURL(),
			bytes.NewReader(body),
		)
		if err != nil {
			return fmt.Errorf(
				"create delete request: %w",
				err,
			)
		}

		s.setHeaders(req)

		req.Header.Set(
			"Content-Type",
			"application/json",
		)

		resp, err := s.httpClient.Do(req)
		if err != nil {
			return fmt.Errorf(
				"delete request failed: %w",
				err,
			)
		}

		responseBody, readErr := io.ReadAll(
			io.LimitReader(resp.Body, 4096),
		)

		resp.Body.Close()

		if readErr != nil {
			return fmt.Errorf(
				"read delete response: %w",
				readErr,
			)
		}

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return fmt.Errorf(
				"delete failed: status=%d body=%s",
				resp.StatusCode,
				string(responseBody),
			)
		}
	}

	return nil
}

// DeleteObject deletes one object.
func (s *SupabaseStorage) DeleteObject(
	objectName string,
) error {

	payload := struct {
		Prefixes []string `json:"prefixes"`
	}{
		Prefixes: []string{
			objectName,
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf(
			"marshal delete object request: %w",
			err,
		)
	}

	req, err := http.NewRequest(
		http.MethodDelete,
		s.deleteURL(),
		bytes.NewReader(body),
	)
	if err != nil {
		return fmt.Errorf(
			"create delete object request: %w",
			err,
		)
	}

	s.setHeaders(req)

	req.Header.Set(
		"Content-Type",
		"application/json",
	)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf(
			"delete object request failed: %w",
			err,
		)
	}

	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {

		responseBody, _ := io.ReadAll(
			io.LimitReader(resp.Body, 4096),
		)

		return fmt.Errorf(
			"delete object failed: status=%d body=%s",
			resp.StatusCode,
			string(responseBody),
		)
	}

	return nil
}
