package store

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// DiskStore manages shard storage on local disk.
type DiskStore struct {
	basePath   string
	mu         sync.RWMutex
	shardCount int64
	usedBytes  int64
}

// NewDiskStore initializes local directory structure and scans existing usage.
func NewDiskStore(basePath string) (*DiskStore, error) {
	if err := os.MkdirAll(basePath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create storage directory: %w", err)
	}

	ds := &DiskStore{basePath: basePath}
	ds.calculateUsage() // Count existing shards on startup
	return ds, nil
}

// shardPath returns the full filesystem path for a shard file.
func (ds *DiskStore) shardPath(fileID, chunkID string, shardIndex int32) string {
	dir := filepath.Join(ds.basePath, fileID)
	return filepath.Join(dir, fmt.Sprintf("%s_%d.shard", chunkID, shardIndex))
}

// WriteShard writes shard data to disk using atomic write-then-rename:
// 1. Verify sha256 checksum before writing.
// 2. Write to a .tmp file.
// 3. Verify size of the temporary file.
// 4. Rename .tmp -> .shard atomically.
func (ds *DiskStore) WriteShard(fileID, chunkID string, shardIndex int32, data []byte, expectedChecksum string) error {
	ds.mu.Lock()
	defer ds.mu.Unlock()

	// Verify checksum before writing
	hash := sha256.Sum256(data)
	actualChecksum := hex.EncodeToString(hash[:])
	if actualChecksum != expectedChecksum {
		return fmt.Errorf("checksum mismatch: expected %s, got %s", expectedChecksum, actualChecksum)
	}

	// Create parent directory for this file
	dir := filepath.Join(ds.basePath, fileID)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create shard directory: %w", err)
	}

	// Step 1: Write to temporary file
	finalPath := ds.shardPath(fileID, chunkID, shardIndex)
	tmpPath := finalPath + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("failed to write temp shard: %w", err)
	}

	// Step 2: Verify the written file size
	info, err := os.Stat(tmpPath)
	if err != nil || info.Size() != int64(len(data)) {
		os.Remove(tmpPath)
		return fmt.Errorf("temp shard verification failed: expected %d bytes, got %d", len(data), info.Size())
	}

	// Step 3: Atomic rename .tmp -> .shard
	if err := os.Rename(tmpPath, finalPath); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("failed to finalize shard (rename): %w", err)
	}

	ds.shardCount++
	ds.usedBytes += int64(len(data))
	return nil
}

// ReadShard reads a shard from disk and returns its bytes.
func (ds *DiskStore) ReadShard(fileID, chunkID string, shardIndex int32) ([]byte, error) {
	ds.mu.RLock()
	defer ds.mu.RUnlock()

	path := ds.shardPath(fileID, chunkID, shardIndex)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("shard not found: %s", path)
		}
		return nil, fmt.Errorf("failed to read shard: %w", err)
	}

	return data, nil
}

// DeleteShard removes a single shard from disk.
func (ds *DiskStore) DeleteShard(fileID, chunkID string, shardIndex int32) error {
	ds.mu.Lock()
	defer ds.mu.Unlock()

	path := ds.shardPath(fileID, chunkID, shardIndex)
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // Already deleted
		}
		return err
	}

	size := info.Size()
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("failed to delete shard: %w", err)
	}

	ds.shardCount--
	ds.usedBytes -= size
	return nil
}

// DeleteFileShards removes all shards for a given file.
func (ds *DiskStore) DeleteFileShards(fileID string) (int32, error) {
	ds.mu.Lock()
	defer ds.mu.Unlock()

	dir := filepath.Join(ds.basePath, fileID)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}

	var count int32
	for _, entry := range entries {
		path := filepath.Join(dir, entry.Name())
		info, _ := entry.Info()
		if info != nil {
			ds.usedBytes -= info.Size()
		}
		if err := os.Remove(path); err == nil {
			count++
			ds.shardCount--
		}
	}

	os.Remove(dir) // Remove empty directory
	return count, nil
}

// Stats returns current storage usage.
func (ds *DiskStore) Stats() (shardCount int64, usedBytes int64) {
	ds.mu.RLock()
	defer ds.mu.RUnlock()
	return ds.shardCount, ds.usedBytes
}

// calculateUsage scans the storage directory to count existing shards.
func (ds *DiskStore) calculateUsage() {
	_ = filepath.Walk(ds.basePath, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() {
			return nil
		}
		if filepath.Ext(path) == ".shard" {
			ds.shardCount++
			ds.usedBytes += info.Size()
		}
		return nil
	})
}
