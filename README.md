# Distributed File Storage Cluster with Reed-Solomon Erasure Coding

A robust distributed file storage system that splits and chunks files across 4 storage nodes with parity shards, ensuring complete data reconstruction upon simulated node failure with zero data loss.

---

## 🏛️ Architecture Overview

```
User (Browser)
      │
      ▼
┌────────────────────────┐
│ React + Vite Dashboard │ (Port 5173 - Dark Mode, Live Simulation Controls)
└───────────┬────────────┘
            │ REST API (JSON / Multipart / Binary)
            ▼
┌────────────────────────┐
│     Go Coordinator     │ (Port 8080 - Chunking, RS 3+1, Orchestration)
└───────┬────────┬───────┘
        │        │
  PostgreSQL 16  │ gRPC Streaming (Client / Server)
(Metadata only)  │
                 ├──► Storage Node 1 (Port 50051 - Data Shard 0)
                 ├──► Storage Node 2 (Port 50052 - Data Shard 1)
                 ├──► Storage Node 3 (Port 50053 - Data Shard 2)
                 └──► Storage Node 4 (Port 50054 - Parity Shard 3)
```

### Technology Stack
- **Go 1.22+**: Core coordinator backend and storage node microservices.
- **gRPC / Protocol Buffers (proto3)**: High-throughput chunk streaming between coordinator and storage nodes.
- **Reed-Solomon Erasure Coding (`klauspost/reedsolomon`)**: 3 Data Shards + 1 Parity Shard ($K=3, M=1$).
- **PostgreSQL 16**: Metadata storage for files, chunks, shards, and node health. Shard raw binaries reside exclusively on node disks.
- **React 18 + Vite + Tailwind CSS**: Real-time simulation dashboard with node start/stop controls.
- **Docker Compose**: Containerized multi-service orchestration.

---

## 🚀 Quick Start

### Prerequisites
- [Docker](https://docs.docker.com/get-docker/) & Docker Compose
- *Optional (for local development without Docker)*: Go 1.22+, Node.js 20+, PostgreSQL 16

### Run with Docker Compose

1. Clone or navigate to the repository directory:
   ```bash
   cd Dist_File_Storeage
   ```

2. Start the entire cluster with Docker Compose:
   ```bash
   docker-compose up --build -d
   ```

3. Verify all 7 containers are healthy and running:
   ```bash
   docker-compose ps
   ```

4. Open the simulation dashboard in your browser:
   ```
   http://localhost:5173
   ```

---

## ⚙️ How It Works

### 1. Upload Flow
1. User uploads a file through the dashboard or `POST /api/files/upload`.
2. Coordinator calculates the original file's **SHA-256** checksum.
3. The file is split into fixed-size **4MB chunks**.
4. Each chunk is Reed-Solomon encoded into **3 data shards + 1 parity shard**.
5. Shards are streamed to the 4 storage nodes via gRPC and stored on disk with checksum verification.
6. Shard and file metadata are saved in PostgreSQL; the file is marked `HEALTHY`.

### 2. Download & Failsafe Reconstruction Flow
1. User requests a download (`GET /api/files/:id/download`).
2. **Preflight Check**: Coordinator verifies online storage nodes.
   - If all nodes are **ONLINE**: Shards are fetched and assembled directly.
   - If 1 node is **OFFLINE**: Coordinator fetches available shards (2 data + 1 parity) and uses **Reed-Solomon decoding** to mathematically reconstruct the missing shard.
   - If &ge; 2 nodes are **OFFLINE**: Failsafe triggers and rejects the download with **HTTP 503 Service Unavailable** (insufficient shards to guarantee reconstruction).
3. **Integrity Validation**: Reconstructed file SHA-256 is checked against the original checksum stored in PostgreSQL.
4. If valid, the file is delivered to the user with `X-Reconstructed: true` and recovery headers. If invalid, the download is aborted to prevent data corruption.

### 3. Failure Simulation
- Each node card in the dashboard features a **Stop / Start** toggle.
- Simulating a node stop marks it `OFFLINE` in the database, mimicking hardware, power, or network loss.
- Stopping 1 node demonstrates transparent Reed-Solomon reconstruction.
- Stopping 2 nodes showcases the failsafe mechanism preventing corrupted downloads.

---

## 📡 API Reference

| Method | Endpoint | Description |
|---|---|---|
| `POST` | `/api/files/upload` | Upload a file (`multipart/form-data`) |
| `GET` | `/api/files` | List all files in the cluster |
| `GET` | `/api/files/:id` | Get metadata and shard placement for a file |
| `GET` | `/api/files/:id/download` | Download or reconstruct a file |
| `DELETE` | `/api/files/:id` | Delete a file and its shards across all nodes |
| `GET` | `/api/nodes` | List all 4 storage nodes and their statuses |
| `GET` | `/api/nodes/:id` | Get status and storage metrics for a node |
| `POST` | `/api/nodes/:id/stop` | Simulate node failure (set to `OFFLINE`) |
| `POST` | `/api/nodes/:id/start` | Restore node to `ONLINE` |
| `GET` | `/api/stats` | Cluster storage and file metrics |
| `GET` | `/api/health` | Coordinator health check |

---

## 🧪 Testing

### Automated Test Suite

#### On Linux / macOS (Bash):
```bash
chmod +x test/integration_test.sh
./test/integration_test.sh
```

#### On Windows (PowerShell):
```powershell
.\test\test_cluster.ps1
```

Both test suites perform automated verification of:
1. Health check & 4-node cluster discovery
2. File upload with chunking & Reed-Solomon distribution
3. Direct download & SHA-256 verification (all nodes online)
4. Simulated node failure (`node-2` stopped)
5. Reconstructed download & SHA-256 verification (1 node offline)
6. Multiple node failure handling (2 nodes offline -> HTTP 503 failsafe)
7. Node restoration to online status
8. Complete file deletion across all storage nodes and database

---

## 📁 Repository Structure

```
Dist_File_Storeage/
├── coordinator/                 # Go Coordinator service
│   ├── cmd/server/main.go       # Entry point
│   ├── internal/
│   │   ├── api/                 # REST handlers & Gin router
│   │   ├── checksum/            # SHA-256 verification utilities
│   │   ├── chunker/             # File splitting and chunk merging
│   │   ├── db/                  # PostgreSQL connection & SQL queries
│   │   ├── erasure/             # Reed-Solomon encoding & decoding
│   │   ├── models/              # Data structures & JSON models
│   │   ├── nodemanager/         # Node tracking & simulation logic
│   │   ├── reconstruction/      # Download reconstruction & integrity validation
│   │   └── shardmanager/        # gRPC client to storage nodes
│   ├── go.mod
│   └── go.sum
├── storage-node/                # Go Storage Node service
│   ├── cmd/node/main.go         # Node entry point
│   ├── internal/
│   │   ├── server/              # gRPC server implementation
│   │   └── store/               # Atomic local disk read/write
│   ├── go.mod
│   └── go.sum
├── proto/                       # Shared Protocol Buffer definitions
│   ├── storage.proto            # gRPC service definition
│   ├── go.mod
│   └── gen/storagepb/           # Generated Go protobuf & gRPC stubs
├── dashboard/                   # React 18 + Vite + Tailwind frontend
│   ├── src/
│   │   ├── components/          # NodePanel, NodeCard, FileUpload, FileList, Layout
│   │   ├── services/            # Axios API client
│   │   ├── App.jsx
│   │   └── main.jsx
│   ├── index.html
│   ├── package.json
│   └── vite.config.js
├── migrations/                  # PostgreSQL DDL migrations
│   ├── 001_create_nodes.sql
│   ├── 002_create_files.sql
│   ├── 003_create_chunks.sql
│   └── 004_create_shards.sql
├── test/                        # Integration test scripts
│   ├── integration_test.sh      # Bash test suite
│   └── test_cluster.ps1         # PowerShell test suite
├── docker-compose.yml           # Complete 7-container cluster orchestration
├── Dockerfile.coordinator       # Coordinator multi-stage Docker build
├── Dockerfile.storage-node      # Storage node multi-stage Docker build
├── Dockerfile.dashboard         # Dashboard Vite production/dev container
├── .env                         # Cluster environment configuration
└── README.md                    # Project documentation
```
