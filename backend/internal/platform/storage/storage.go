package storage

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"taskmgmt/internal/config"
)

type StorageService interface {
	Driver() string
	PresignUpload(ctx context.Context, objectKey, contentType string, expiry time.Duration) (string, error)
	PresignDownload(ctx context.Context, objectKey string, expiry time.Duration) (string, error)
	UploadStream(ctx context.Context, objectKey string, reader io.Reader, size int64, contentType string) error
	Open(ctx context.Context, objectKey string) (io.ReadCloser, error)
	Delete(ctx context.Context, objectKey string) error
}

func NewStorage(cfg *config.Config) (StorageService, error) {
	driver := strings.ToLower(strings.TrimSpace(cfg.StorageDriver))
	if driver == "minio" || driver == "s3" {
		minioStorage, err := newMinIOStorage(cfg)
		if err == nil {
			return minioStorage, nil
		}
		// Fallback to local storage if MinIO is not reachable (e.g., local tests)
		fmt.Printf("[Storage] MinIO initialization failed (%v), falling back to local storage\n", err)
	}

	uploadDir := cfg.UploadDir
	if uploadDir == "" {
		uploadDir = "./uploads"
	}
	return newLocalStorage(uploadDir), nil
}

// --- Local Storage Implementation ---

type LocalStorage struct {
	baseDir string
}

func newLocalStorage(baseDir string) *LocalStorage {
	_ = os.MkdirAll(baseDir, 0755)
	return &LocalStorage{baseDir: baseDir}
}

func (l *LocalStorage) Driver() string {
	return "local"
}

func (l *LocalStorage) getFullPath(objectKey string) string {
	cleanKey := filepath.Clean("/" + objectKey)
	return filepath.Join(l.baseDir, cleanKey)
}

func (l *LocalStorage) PresignUpload(ctx context.Context, objectKey, contentType string, expiry time.Duration) (string, error) {
	// For local storage, client should perform direct multipart POST
	return "", nil
}

func (l *LocalStorage) PresignDownload(ctx context.Context, objectKey string, expiry time.Duration) (string, error) {
	// For local storage, backend streams directly
	return "", nil
}

func (l *LocalStorage) UploadStream(ctx context.Context, objectKey string, reader io.Reader, size int64, contentType string) error {
	fullPath := l.getFullPath(objectKey)
	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		return err
	}

	dst, err := os.Create(fullPath)
	if err != nil {
		return err
	}
	defer dst.Close()

	_, err = io.Copy(dst, reader)
	return err
}

func (l *LocalStorage) Open(ctx context.Context, objectKey string) (io.ReadCloser, error) {
	fullPath := l.getFullPath(objectKey)
	return os.Open(fullPath)
}

func (l *LocalStorage) Delete(ctx context.Context, objectKey string) error {
	fullPath := l.getFullPath(objectKey)
	err := os.Remove(fullPath)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

// --- MinIO / S3 Implementation ---

type MinIOStorage struct {
	client *minio.Client
	bucket string
}

func newMinIOStorage(cfg *config.Config) (*MinIOStorage, error) {
	client, err := minio.New(cfg.S3Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.S3AccessKey, cfg.S3SecretKey, ""),
		Secure: cfg.S3UseSSL,
	})
	if err != nil {
		return nil, err
	}

	bucket := cfg.S3Bucket
	if bucket == "" {
		bucket = "taskmgmt-attachments"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	exists, err := client.BucketExists(ctx, bucket)
	if err == nil && !exists {
		_ = client.MakeBucket(ctx, bucket, minio.MakeBucketOptions{})
	}

	return &MinIOStorage{
		client: client,
		bucket: bucket,
	}, nil
}

func (m *MinIOStorage) Driver() string {
	return "minio"
}

func (m *MinIOStorage) PresignUpload(ctx context.Context, objectKey, contentType string, expiry time.Duration) (string, error) {
	reqParams := make(url.Values)
	if contentType != "" {
		reqParams.Set("response-content-type", contentType)
	}

	u, err := m.client.PresignedPutObject(ctx, m.bucket, objectKey, expiry)
	if err != nil {
		return "", err
	}
	return u.String(), nil
}

func (m *MinIOStorage) PresignDownload(ctx context.Context, objectKey string, expiry time.Duration) (string, error) {
	reqParams := make(url.Values)
	u, err := m.client.PresignedGetObject(ctx, m.bucket, objectKey, expiry, reqParams)
	if err != nil {
		return "", err
	}
	return u.String(), nil
}

func (m *MinIOStorage) UploadStream(ctx context.Context, objectKey string, reader io.Reader, size int64, contentType string) error {
	_, err := m.client.PutObject(ctx, m.bucket, objectKey, reader, size, minio.PutObjectOptions{
		ContentType: contentType,
	})
	return err
}

func (m *MinIOStorage) Open(ctx context.Context, objectKey string) (io.ReadCloser, error) {
	obj, err := m.client.GetObject(ctx, m.bucket, objectKey, minio.GetObjectOptions{})
	if err != nil {
		return nil, err
	}
	return obj, nil
}

func (m *MinIOStorage) Delete(ctx context.Context, objectKey string) error {
	return m.client.RemoveObject(ctx, m.bucket, objectKey, minio.RemoveObjectOptions{})
}

