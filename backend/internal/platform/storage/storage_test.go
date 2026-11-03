package storage

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"taskmgmt/internal/config"
)

func TestLocalStorage(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "taskmgmt-storage-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	cfg := &config.Config{
		StorageDriver: "local",
		UploadDir:     tempDir,
	}

	svc, err := NewStorage(cfg)
	require.NoError(t, err)
	assert.Equal(t, "local", svc.Driver())

	ctx := context.Background()
	objKey := "org-1/task-1/sample.txt"
	content := []byte("hello storage test")

	// Test Upload
	err = svc.UploadStream(ctx, objKey, bytes.NewReader(content), int64(len(content)), "text/plain")
	assert.NoError(t, err)

	// Verify file exists on disk
	fullPath := filepath.Join(tempDir, objKey)
	_, err = os.Stat(fullPath)
	assert.NoError(t, err)

	// Test Open / Read
	rc, err := svc.Open(ctx, objKey)
	require.NoError(t, err)
	readBytes, err := io.ReadAll(rc)
	rc.Close()
	assert.NoError(t, err)
	assert.Equal(t, content, readBytes)

	// Test Delete
	err = svc.Delete(ctx, objKey)
	assert.NoError(t, err)

	_, err = os.Stat(fullPath)
	assert.True(t, os.IsNotExist(err))
}

