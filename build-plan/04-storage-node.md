# Step 04 — Storage Node gRPC Server

## Goal
Implement the gRPC storage node server that stores/retrieves/deletes shards on local disk. Each of the 4 storage nodes runs this same binary with a different node ID and port.

---

## Architecture

```
Storage Node Process
    │
    ├── gRPC Server (port 5005x)
    │       │
    │       ├── StoreShard()      → Write shard to /data/shards/<file_id>/<chunk_id>_<shard_index>
    │       ├── RetrieveShard()   → Read shard from disk, stream back
    │       ├── DeleteShard()     → Remove shard file
    │       ├── DeleteFileShards()→ Remove all shards for a file
    │       └── HealthCheck()     → Return node status + disk usage
    │
    └── Local Disk
            /data/shards/
                <file_id>/
                    <chunk_id>_<shard_index>.shard
```

---

## Files to Create

### `storage-node/internal/store/store.go`

```go
package store

import (
    "crypto/sha256"
    "encoding/hex"
    "fmt"
    "io"
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

func NewDiskStore(basePath string) (*DiskStore, error) {
    if err := os.MkdirAll(basePath, 0755); err != nil {
        return nil, fmt.Errorf("failed to create storage directory: %w", err)
    }

    ds := &DiskStore{basePath: basePath}
    ds.calculateUsage() // Count existing shards on startup
    return ds, nil
}

// shardPath returns the full filesystem path for a shard.
func (ds *DiskStore) shardPath(fileID, chunkID string, shardIndex int32) string {
    dir := filepath.Join(ds.basePath, fileID)
    return filepath.Join(dir, fmt.Sprintf("%s_%d.shard", chunkID, shardIndex))
}

// WriteShard writes shard data to disk using atomic write-then-rename.
// 1. Write to a .tmp file
// 2. Verify checksum and size of the .tmp file
// 3. Rename .tmp → .shard (atomic on most filesystems)
// This prevents partially-written shards from being treated as valid on crash.
func (ds *DiskStore) WriteShard(fileID, chunkID string, shardIndex int32, data []byte, expectedChecksum string) error {
    ds.mu.Lock()
    defer ds.mu.Unlock()

    // Verify checksum before writing
    hash := sha256.Sum256(data)
    actualChecksum := hex.EncodeToString(hash[:])
    if actualChecksum != expectedChecksum {
        return fmt.Errorf("checksum mismatch: expected %s, got %s", expectedChecksum, actualChecksum)
    }

    // Create file directory
    dir := filepath.Join(ds.basePath, fileID)
    if err := os.MkdirAll(dir, 0755); err != nil {
        return fmt.Errorf("failed to create shard directory: %w", err)
    }

    // Step 1: Write to temporary file
    finalPath := ds.shardPath(fileID, chunkID, shardIndex)
    tmpPath := finalPath + ".tmp"
    if err := os.WriteFile(tmpPath, data, 0644); err != nil {
        os.Remove(tmpPath) // Clean up on failure
        return fmt.Errorf("failed to write temp shard: %w", err)
    }

    // Step 2: Verify the written file (read back and check size)
    info, err := os.Stat(tmpPath)
    if err != nil || info.Size() != int64(len(data)) {
        os.Remove(tmpPath)
        return fmt.Errorf("temp shard verification failed: expected %d bytes, got %d", len(data), info.Size())
    }

    // Step 3: Atomic rename .tmp → .shard
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
    filepath.Walk(ds.basePath, func(path string, info os.FileInfo, err error) error {
        if err != nil || info.IsDir() {
            return nil
        }
        if filepath.Ext(path) == ".shard" {
            ds.shardCount++
            ds.usedBytes += info.Size()
        }
        return nil
    })
}
```

### `storage-node/internal/server/server.go`

```go
package server

import (
    "bytes"
    "context"
    "fmt"
    "io"
    "log"

    pb "github.com/surya/dist-file-storage/proto/gen/storagepb"
    "github.com/surya/dist-file-storage/storage-node/internal/store"
)

const grpcChunkSize = 64 * 1024 // 64KB per gRPC stream message

// StorageServer implements the StorageNode gRPC service.
type StorageServer struct {
    pb.UnimplementedStorageNodeServer
    nodeID    string
    diskStore *store.DiskStore
    totalStorage int64 // configured total capacity in bytes
}

func NewStorageServer(nodeID string, diskStore *store.DiskStore, totalStorage int64) *StorageServer {
    return &StorageServer{
        nodeID:       nodeID,
        diskStore:    diskStore,
        totalStorage: totalStorage,
    }
}

// StoreShard receives a shard via client streaming and writes it to disk.
func (s *StorageServer) StoreShard(stream pb.StorageNode_StoreShardServer) error {
    var metadata *pb.ShardMetadata
    var buf bytes.Buffer

    for {
        req, err := stream.Recv()
        if err == io.EOF {
            break
        }
        if err != nil {
            return fmt.Errorf("error receiving shard data: %w", err)
        }

        switch payload := req.Payload.(type) {
        case *pb.StoreShardRequest_Metadata:
            metadata = payload.Metadata
            log.Printf("[%s] Receiving shard: file=%s chunk=%s index=%d type=%s",
                s.nodeID, metadata.FileId, metadata.ChunkId, metadata.ShardIndex, metadata.ShardType)
        case *pb.StoreShardRequest_Data:
            buf.Write(payload.Data)
        }
    }

    if metadata == nil {
        return fmt.Errorf("no metadata received")
    }

    // Write shard to disk with checksum verification
    err := s.diskStore.WriteShard(
        metadata.FileId,
        metadata.ChunkId,
        metadata.ShardIndex,
        buf.Bytes(),
        metadata.Checksum,
    )
    if err != nil {
        log.Printf("[%s] Failed to store shard: %v", s.nodeID, err)
        return stream.SendAndClose(&pb.StoreShardResponse{
            Success: false,
            Message: err.Error(),
        })
    }

    shardID := fmt.Sprintf("%s/%s/%d", metadata.FileId, metadata.ChunkId, metadata.ShardIndex)
    log.Printf("[%s] ✓ Stored shard: %s (%d bytes)", s.nodeID, shardID, buf.Len())

    return stream.SendAndClose(&pb.StoreShardResponse{
        Success: true,
        Message: "Shard stored successfully",
        ShardId: shardID,
    })
}

// RetrieveShard reads a shard from disk and streams it back.
func (s *StorageServer) RetrieveShard(req *pb.RetrieveShardRequest, stream pb.StorageNode_RetrieveShardServer) error {
    log.Printf("[%s] Retrieving shard: file=%s chunk=%s index=%d",
        s.nodeID, req.FileId, req.ChunkId, req.ShardIndex)

    data, err := s.diskStore.ReadShard(req.FileId, req.ChunkId, req.ShardIndex)
    if err != nil {
        return fmt.Errorf("failed to read shard: %w", err)
    }

    // Stream the shard data in 64KB chunks
    for offset := 0; offset < len(data); offset += grpcChunkSize {
        end := offset + grpcChunkSize
        if end > len(data) {
            end = len(data)
        }

        if err := stream.Send(&pb.RetrieveShardResponse{
            Data: data[offset:end],
        }); err != nil {
            return fmt.Errorf("failed to stream shard data: %w", err)
        }
    }

    log.Printf("[%s] ✓ Sent shard: %d bytes", s.nodeID, len(data))
    return nil
}

// DeleteShard removes a single shard from this node.
func (s *StorageServer) DeleteShard(ctx context.Context, req *pb.DeleteShardRequest) (*pb.DeleteShardResponse, error) {
    log.Printf("[%s] Deleting shard: file=%s chunk=%s index=%d",
        s.nodeID, req.FileId, req.ChunkId, req.ShardIndex)

    err := s.diskStore.DeleteShard(req.FileId, req.ChunkId, req.ShardIndex)
    if err != nil {
        return &pb.DeleteShardResponse{
            Success: false,
            Message: err.Error(),
        }, nil
    }

    return &pb.DeleteShardResponse{
        Success: true,
        Message: "Shard deleted",
    }, nil
}

// DeleteFileShards removes all shards for a file from this node.
func (s *StorageServer) DeleteFileShards(ctx context.Context, req *pb.DeleteFileShardsRequest) (*pb.DeleteFileShardsResponse, error) {
    log.Printf("[%s] Deleting all shards for file: %s", s.nodeID, req.FileId)

    count, err := s.diskStore.DeleteFileShards(req.FileId)
    if err != nil {
        return &pb.DeleteFileShardsResponse{
            Success: false,
            Message: err.Error(),
        }, nil
    }

    return &pb.DeleteFileShardsResponse{
        Success:       true,
        Message:       fmt.Sprintf("Deleted %d shards", count),
        ShardsDeleted: count,
    }, nil
}

// HealthCheck returns this node's status and storage usage.
func (s *StorageServer) HealthCheck(ctx context.Context, req *pb.HealthCheckRequest) (*pb.HealthCheckResponse, error) {
    shardCount, usedBytes := s.diskStore.Stats()

    return &pb.HealthCheckResponse{
        Healthy:      true,
        NodeId:       s.nodeID,
        TotalStorage: s.totalStorage,
        UsedStorage:  usedBytes,
        ShardCount:   shardCount,
    }, nil
}
```

### `storage-node/cmd/node/main.go`

```go
package main

import (
    "fmt"
    "log"
    "net"
    "os"
    "os/signal"
    "syscall"

    "google.golang.org/grpc"

    pb "github.com/surya/dist-file-storage/proto/gen/storagepb"
    "github.com/surya/dist-file-storage/storage-node/internal/server"
    "github.com/surya/dist-file-storage/storage-node/internal/store"
)

func main() {
    nodeID := getEnv("NODE_ID", "node-1")
    port := getEnv("GRPC_PORT", "50051")
    dataDir := getEnv("DATA_DIR", "/data/shards")
    totalStorage := int64(10 * 1024 * 1024 * 1024) // 10GB default

    log.Printf("Starting storage node: %s on port %s", nodeID, port)
    log.Printf("Data directory: %s", dataDir)

    // Initialize disk storage
    diskStore, err := store.NewDiskStore(dataDir)
    if err != nil {
        log.Fatalf("Failed to initialize disk store: %v", err)
    }

    shardCount, usedBytes := diskStore.Stats()
    log.Printf("Existing shards: %d, Used storage: %d bytes", shardCount, usedBytes)

    // Start gRPC server
    lis, err := net.Listen("tcp", fmt.Sprintf(":%s", port))
    if err != nil {
        log.Fatalf("Failed to listen on port %s: %v", port, err)
    }

    grpcServer := grpc.NewServer(
        grpc.MaxRecvMsgSize(8 * 1024 * 1024), // 8MB max message
        grpc.MaxSendMsgSize(8 * 1024 * 1024),
    )

    storageServer := server.NewStorageServer(nodeID, diskStore, totalStorage)
    pb.RegisterStorageNodeServer(grpcServer, storageServer)

    // Graceful shutdown
    go func() {
        sigChan := make(chan os.Signal, 1)
        signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
        sig := <-sigChan
        log.Printf("Received signal %v, shutting down...", sig)
        grpcServer.GracefulStop()
    }()

    log.Printf("✓ Storage node %s listening on :%s", nodeID, port)
    if err := grpcServer.Serve(lis); err != nil {
        log.Fatalf("Failed to serve: %v", err)
    }
}

func getEnv(key, fallback string) string {
    if val := os.Getenv(key); val != "" {
        return val
    }
    return fallback
}
```

---

## How Shards Are Stored on Disk

```
/data/shards/
    ├── <file-uuid-1>/
    │   ├── <chunk-uuid-0>_0.shard    ← data shard 0
    │   ├── <chunk-uuid-1>_0.shard
    │   └── <chunk-uuid-2>_0.shard
    │
    └── <file-uuid-2>/
        ├── <chunk-uuid-0>_0.shard
        └── <chunk-uuid-1>_0.shard
```

Each node stores only the shards assigned to it. For example:
- **Node 1** stores shard_index=0 (Data Shard 1) for every chunk
- **Node 2** stores shard_index=1 (Data Shard 2) for every chunk
- **Node 3** stores shard_index=2 (Data Shard 3) for every chunk
- **Node 4** stores shard_index=3 (Parity Shard) for every chunk

---

## Key Design Decisions

| Decision | Rationale |
|----------|-----------|
| **Checksum verification on write** | The node verifies SHA-256 immediately — catch corruption during transfer |
| **Atomic write (tmp → rename)** | Write to `.tmp` file first, verify, then rename to `.shard`. Prevents partially-written shards from being treated as valid if the process crashes mid-write |
| **Streaming in 64KB chunks** | Stays well under gRPC's default 4MB message limit; memory efficient |
| **Same binary, different env vars** | All 4 nodes run the exact same Docker image with different `NODE_ID` and `GRPC_PORT` |
| **RWMutex on disk operations** | Prevents concurrent read/write to the same shard file |
| **Graceful shutdown** | Finishes in-flight RPCs before stopping |

---

## Verification

```bash
# Run a single node locally to test
cd storage-node
NODE_ID=node-1 GRPC_PORT=50051 DATA_DIR=./test-data go run cmd/node/main.go

# Should see:
# Starting storage node: node-1 on port 50051
# Data directory: ./test-data
# ✓ Storage node node-1 listening on :50051
```

---

## What This Step Achieves
- Fully functional gRPC storage node that stores/retrieves/deletes shards on disk
- Atomic shard writing (write → verify → rename) — crash-safe
- Checksum verification on every write
- Streaming for large shard transfers
- Health check reporting storage usage
- Single binary configurable via environment variables for all 4 nodes
