# Step 01 — Project Directory Structure & Go Modules

## Goal
Set up the complete project directory structure, initialize Go modules, and install all Go dependencies.

---

## Final Directory Tree

```
Dist_File_Storeage/
├── build-plan/                    # (this folder — build guides)
├── coordinator/                   # Go backend (REST + business logic)
│   ├── cmd/
│   │   └── server/
│   │       └── main.go            # Entry point
│   ├── internal/
│   │   ├── api/                   # REST handlers
│   │   │   ├── handlers.go
│   │   │   └── router.go
│   │   ├── chunker/               # File chunking logic
│   │   │   └── chunker.go
│   │   ├── erasure/               # Reed-Solomon encode/decode
│   │   │   └── erasure.go
│   │   ├── checksum/              # SHA-256 hashing
│   │   │   └── checksum.go
│   │   ├── nodemanager/           # Node health & simulation
│   │   │   └── manager.go
│   │   ├── shardmanager/          # Shard distribution & retrieval via gRPC
│   │   │   └── manager.go
│   │   ├── reconstruction/        # Reconstruct missing shards + file
│   │   │   └── reconstruct.go
│   │   ├── db/                    # PostgreSQL connection & queries
│   │   │   ├── connection.go
│   │   │   └── queries.go
│   │   └── models/                # Shared data structures
│   │       └── models.go
│   ├── go.mod
│   └── go.sum
│
├── storage-node/                  # Go gRPC storage node server
│   ├── cmd/
│   │   └── node/
│   │       └── main.go            # Entry point
│   ├── internal/
│   │   ├── server/                # gRPC server implementation
│   │   │   └── server.go
│   │   └── store/                 # Local disk read/write
│   │       └── store.go
│   ├── go.mod
│   └── go.sum
│
├── proto/                         # Shared protobuf module (its own go.mod)
│   ├── go.mod
│   ├── storage.proto
│   └── gen/
│       └── storagepb/
│           ├── storage.pb.go      # Generated
│           └── storage_grpc.pb.go # Generated
│
├── dashboard/                     # React + Vite + Tailwind frontend
│   ├── src/
│   │   ├── components/
│   │   │   ├── FileUpload.jsx
│   │   │   ├── FileList.jsx
│   │   │   ├── NodePanel.jsx
│   │   │   ├── NodeCard.jsx
│   │   │   └── Layout.jsx
│   │   ├── pages/
│   │   │   ├── Dashboard.jsx
│   │   │   └── Files.jsx
│   │   ├── services/
│   │   │   └── api.js
│   │   ├── App.jsx
│   │   ├── main.jsx
│   │   └── index.css
│   ├── index.html
│   ├── package.json
│   ├── vite.config.js
│   └── tailwind.config.js
│
├── migrations/                    # SQL migration files
│   ├── 001_create_nodes.sql
│   ├── 002_create_files.sql
│   ├── 003_create_chunks.sql
│   └── 004_create_shards.sql
│
├── docker-compose.yml
├── Dockerfile.coordinator
├── Dockerfile.storage-node
├── Dockerfile.dashboard
├── .env
└── README.md
```

---

## Commands to Execute

### 1. Create directory skeleton

```bash
# From project root: c:\Users\surya\Desktop\Dist_File_Storeage

# Coordinator
mkdir -p coordinator/cmd/server
mkdir -p coordinator/internal/{api,chunker,erasure,checksum,nodemanager,shardmanager,reconstruction,db,models}

# Storage Node
mkdir -p storage-node/cmd/node
mkdir -p storage-node/internal/{server,store}

# Proto
mkdir -p proto

# Migrations
mkdir -p migrations

# Dashboard (will be scaffolded by Vite later)
```

### 2. Initialize Go modules

```bash
# Coordinator module
cd coordinator
go mod init github.com/surya/dist-file-storage/coordinator

# Storage node module
cd ../storage-node
go mod init github.com/surya/dist-file-storage/storage-node
```

### 3. Install Go dependencies

#### Coordinator

```bash
cd coordinator

# Web framework
go get github.com/gin-gonic/gin

# PostgreSQL driver
go get github.com/lib/pq

# Reed-Solomon erasure coding
go get github.com/klauspost/reedsolomon

# gRPC client
go get google.golang.org/grpc
go get google.golang.org/protobuf

# UUID generation
go get github.com/google/uuid

# Environment config
go get github.com/joho/godotenv

# CORS middleware
go get github.com/gin-contrib/cors
```

#### Storage Node

```bash
cd storage-node

# gRPC server
go get google.golang.org/grpc
go get google.golang.org/protobuf

# UUID
go get github.com/google/uuid
```

### 4. Install protoc tools

```bash
# Install protoc compiler (if not installed)
# Windows: download from https://github.com/protocolbuffers/protobuf/releases

# Install Go protobuf plugins
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
```

---

## Files to Create in This Step

### `coordinator/cmd/server/main.go` (placeholder)

```go
package main

import "fmt"

func main() {
    fmt.Println("Coordinator starting...")
}
```

### `storage-node/cmd/node/main.go` (placeholder)

```go
package main

import "fmt"

func main() {
    fmt.Println("Storage node starting...")
}
```

### `.env` (project root)

```env
# PostgreSQL
POSTGRES_HOST=localhost
POSTGRES_PORT=5432
POSTGRES_USER=distfs
POSTGRES_PASSWORD=distfs_secret
POSTGRES_DB=distfs

# Coordinator
COORDINATOR_PORT=8080
CHUNK_SIZE=4194304

# Storage Nodes
NODE1_GRPC_PORT=50051
NODE2_GRPC_PORT=50052
NODE3_GRPC_PORT=50053
NODE4_GRPC_PORT=50054

# Reed-Solomon
DATA_SHARDS=3
PARITY_SHARDS=1
```

---

## Verification

```bash
# From coordinator/
go build ./...
# Should compile without errors

# From storage-node/
go build ./...
# Should compile without errors
```

---

## What This Step Achieves
- Clean, well-organized monorepo with separate Go modules
- All dependencies installed and pinned in `go.mod`
- Project ready for protobuf definitions (Step 02)
