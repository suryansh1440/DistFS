# Step 12 — Final Polish, Error Handling & README

## Goal
Add robust error handling, logging, edge case coverage, and a comprehensive README.

---

## 1. Edge Cases to Handle

### Coordinator — Upload Edge Cases

```go
// In handlers.go UploadFile:

// Edge case: Empty file
if len(fileData) == 0 {
    c.JSON(http.StatusBadRequest, gin.H{"error": "Cannot upload an empty file"})
    return
}

// Edge case: File too large (e.g., > 500MB for demo)
const maxFileSize = 500 * 1024 * 1024 // 500MB
if len(fileData) > maxFileSize {
    c.JSON(http.StatusBadRequest, gin.H{
        "error": fmt.Sprintf("File too large: %d bytes (max %d)", len(fileData), maxFileSize),
    })
    return
}

// Edge case: Not all nodes online during upload
nodes, _ := db.ListNodes()
onlineCount := 0
for _, n := range nodes {
    if n.Status == "ONLINE" {
        onlineCount++
    }
}
if onlineCount < 4 {
    c.JSON(http.StatusServiceUnavailable, gin.H{
        "error":  "Cannot upload: not all storage nodes are online",
        "online": onlineCount,
        "required": 4,
    })
    return
}
```

### Storage Node — Disk Space Check

```go
// In store.go WriteShard, before writing:

// Check available disk space (basic check)
var stat syscall.Statfs_t
if err := syscall.Statfs(ds.basePath, &stat); err == nil {
    availableBytes := stat.Bavail * uint64(stat.Bsize)
    if availableBytes < uint64(len(data)) {
        return fmt.Errorf("insufficient disk space: need %d, have %d", len(data), availableBytes)
    }
}
```

### Coordinator — gRPC Connection Retry

```go
// In shardmanager/manager.go ConnectToNodes:
// Add retry logic with backoff

func (sm *ShardManager) ConnectToNodesWithRetry(maxRetries int) error {
    nodes, err := db.ListNodes()
    if err != nil {
        return fmt.Errorf("failed to list nodes: %w", err)
    }

    for _, node := range nodes {
        var conn *grpc.ClientConn
        var lastErr error

        for attempt := 0; attempt < maxRetries; attempt++ {
            conn, lastErr = grpc.Dial(
                node.GRPCAddress,
                grpc.WithTransportCredentials(insecure.NewCredentials()),
                grpc.WithBlock(),
                grpc.WithTimeout(5 * time.Second),
            )
            if lastErr == nil {
                break
            }
            log.Printf("  Retry %d/%d for %s: %v", attempt+1, maxRetries, node.ID, lastErr)
            time.Sleep(time.Duration(attempt+1) * time.Second)
        }

        if lastErr != nil {
            log.Printf("⚠ Failed to connect to %s after %d retries", node.ID, maxRetries)
            continue
        }

        sm.connections[node.ID] = pb.NewStorageNodeClient(conn)
        log.Printf("✓ Connected to %s at %s", node.ID, node.GRPCAddress)
    }

    return nil
}
```

---

## 2. Structured Logging

Add consistent log formatting throughout the coordinator:

```go
// Prefix all log lines:
// [UPLOAD]   file operations
// [DOWNLOAD] download/reconstruction
// [NODE]     node management
// [SHARD]    shard distribution
// [RS]       Reed-Solomon encoding/decoding
// [DB]       database operations

log.Printf("[UPLOAD] Starting: %s (%d bytes)", header.Filename, len(fileData))
log.Printf("[RS] Encoding chunk %d: %d bytes → %d shards", chk.Index, len(chk.Data), totalShards)
log.Printf("[SHARD] Storing shard %d → %s via gRPC", shardIdx, nodeID)
log.Printf("[DOWNLOAD] Reconstruction: %d missing, %d recovered", missing, recovered)
log.Printf("[NODE] %s → %s", nodeID, newStatus)
```

---

## 3. README.md

Create `README.md` in the project root:

```markdown
# Distributed File Storage Cluster with Reed-Solomon Erasure Coding

A distributed file storage system that chunks files across 4 storage nodes with
parity shards, ensuring complete data reconstruction upon simulated node failure.

## Architecture

```
User → React Dashboard → REST API → Go Coordinator → gRPC → 4 Storage Nodes
                                         ↓
                                    PostgreSQL
                                   (metadata only)
```

| Component       | Technology                   |
|----------------|------------------------------|
| Frontend        | React + Vite + Tailwind CSS  |
| Backend         | Go + Gin (REST API)          |
| Storage Nodes   | Go + gRPC                    |
| Metadata Store  | PostgreSQL 16                |
| Erasure Coding  | Reed-Solomon (3 data + 1 parity) |
| Orchestration   | Docker Compose               |

## Quick Start

```bash
# Clone the repository
git clone <repo-url>
cd Dist_File_Storeage

# Build and start all services
docker-compose up --build -d

# Verify everything is running
docker-compose ps

# Open the dashboard
open http://localhost:5173
```

## How It Works

### Upload Flow
1. User uploads a file via the React dashboard
2. Go coordinator calculates SHA-256 checksum of the original file
3. File is split into 4MB chunks
4. Each chunk is Reed-Solomon encoded into 3 data shards + 1 parity shard
5. Shards are distributed to 4 storage nodes via gRPC
6. Metadata (file, chunk, shard info) is stored in PostgreSQL

### Download Flow
1. User requests a file download
2. Coordinator checks node availability
3. If all nodes online: fetch data shards directly
4. If a node is offline: fetch available shards + reconstruct missing ones via Reed-Solomon
5. Reassemble chunks into the original file
6. Verify SHA-256 checksum against the original
7. If valid: send file to user. If invalid: return error (failsafe)

### Node Failure Simulation
- Dashboard provides Stop/Start buttons for each node
- Stopping a node marks it OFFLINE — the coordinator skips it during operations
- With 1 node offline: downloads work via Reed-Solomon reconstruction
- With 2+ nodes offline: downloads are refused (not enough shards)

## Reed-Solomon Configuration

| Parameter     | Value |
|--------------|-------|
| Data Shards (K) | 3  |
| Parity Shards (M) | 1 |
| Chunk Size    | 4 MB  |
| Fault Tolerance | 1 node failure |

## API Endpoints

| Method | Path | Description |
|--------|------|-------------|
| POST   | /api/files/upload | Upload a file |
| GET    | /api/files | List all files |
| GET    | /api/files/:id | Get file details |
| GET    | /api/files/:id/download | Download a file |
| DELETE | /api/files/:id | Delete a file |
| GET    | /api/nodes | List storage nodes |
| POST   | /api/nodes/:id/stop | Simulate node failure |
| POST   | /api/nodes/:id/start | Bring node back online |
| GET    | /api/stats | Cluster statistics |
| GET    | /api/health | Health check |

## Testing

```bash
# Run integration tests
./test/integration_test.sh
```

## Services

| Service | Port | Description |
|---------|------|-------------|
| Dashboard | 5173 | React UI |
| Coordinator | 8080 | REST API |
| Node 1 | 50051 | gRPC storage |
| Node 2 | 50052 | gRPC storage |
| Node 3 | 50053 | gRPC storage |
| Node 4 | 50054 | gRPC storage |
| PostgreSQL | 5432 | Metadata DB |

## Project Structure

```
├── coordinator/          # Go backend (REST + logic)
│   ├── cmd/server/       # Entry point
│   └── internal/         # Core modules
│       ├── api/          # REST handlers
│       ├── chunker/      # File chunking
│       ├── erasure/      # Reed-Solomon
│       ├── checksum/     # SHA-256
│       ├── nodemanager/  # Node health
│       ├── shardmanager/ # gRPC shard distribution
│       ├── reconstruction/ # File reconstruction
│       ├── db/           # PostgreSQL
│       └── models/       # Data structures
├── storage-node/         # Go gRPC storage node
├── dashboard/            # React + Tailwind frontend
├── proto/                # Protobuf definitions
├── migrations/           # SQL migrations
├── test/                 # Integration tests
└── docker-compose.yml    # Orchestration
```

## Key Design Decisions

1. **PostgreSQL stores metadata only** — actual shard bytes live on each node's local disk
2. **No automatic recovery** — reconstruction happens on-demand during download
3. **Soft simulation** — nodes are marked offline in DB; gRPC servers remain running
4. **Failsafe** — SHA-256 verified at shard level AND file level before sending to user
5. **gRPC streaming** — large shards are transferred in 64KB stream chunks
```

---

## 4. .gitignore

```gitignore
# Go
coordinator/coordinator
storage-node/storage-node
*.exe
*.exe~
*.dll
*.so
*.dylib

# Dependencies
vendor/

# Generated protobuf
coordinator/gen/
storage-node/gen/

# Node
dashboard/node_modules/
dashboard/dist/

# Environment
.env.local

# IDE
.idea/
.vscode/
*.swp
*.swo

# Docker
postgres_data/

# Test artifacts
/tmp/test_*
```

---

## Final Checklist

| # | Item | Status |
|---|------|--------|
| 1 | Project structure created | ☐ |
| 2 | Protobuf defined & generated | ☐ |
| 3 | PostgreSQL schema & migrations | ☐ |
| 4 | Storage node gRPC server | ☐ |
| 5 | Coordinator core (chunker, RS, checksum) | ☐ |
| 6 | REST API & shard manager | ☐ |
| 7 | Node manager & coordinator main | ☐ |
| 8 | Download/reconstruction flow | ☐ |
| 9 | Docker Compose & Dockerfiles | ☐ |
| 10 | React dashboard | ☐ |
| 11 | Integration tests | ☐ |
| 12 | README & polish | ☐ |

---

## What This Step Achieves
- Robust error handling for edge cases
- Consistent structured logging
- Comprehensive README documentation
- .gitignore for the project
- Final checklist for completion tracking
