# Distributed File Storage Cluster — Build Plan Overview

## Project Summary
Build a **Distributed File Storage Cluster with Reed-Solomon Erasure Coding** using Go (coordinator + storage nodes), PostgreSQL (metadata), gRPC (shard transfer), REST (client API), and React + Tailwind (simulation dashboard).

---

## Architecture at a Glance

```
USER ──► React Dashboard ──REST──► Go Coordinator ──gRPC──► 4 Storage Nodes
                                        │
                                   PostgreSQL
                                   (metadata only)
```

| Layer            | Technology                          | Role                                      |
|------------------|-------------------------------------|--------------------------------------------|
| Frontend         | Vite + React + Tailwind CSS v4 (JS) | Upload, Download, Node Dashboard           |
| Backend API      | Go + Gin/Chi (REST)                 | Coordinator: chunking, RS, validation      |
| Metadata Store   | PostgreSQL 16                       | File, chunk, shard, node metadata          |
| Shard Transfer   | gRPC (protobuf)                     | Send/retrieve shards to/from storage nodes |
| Storage Nodes    | Go gRPC servers                     | Receive shard → write to disk → return     |
| Erasure Coding   | `klauspost/reedsolomon`             | 3 data + 1 parity shards per chunk         |
| Checksums        | SHA-256                             | File-level + shard-level integrity         |
| Orchestration    | Docker Compose                      | 1 coordinator + 4 nodes + 1 postgres + 1 frontend |

---

## Reed-Solomon Configuration

| Parameter    | Value |
|-------------|-------|
| Data shards  | 3 (K) |
| Parity shards| 1 (M) |
| Total shards | 4     |
| Chunk size   | 4 MB  |
| Tolerance    | 1 node failure per chunk |

---

## Build Phases (one MD file each)

| Step | File                          | Title                                      |
|------|-------------------------------|--------------------------------------------|
| 01   | `01-project-structure.md`     | Project Directory Structure & Go Modules   |
| 02   | `02-protobuf-grpc.md`         | Protobuf Definitions & gRPC Code Gen       |
| 03   | `03-postgresql-schema.md`     | PostgreSQL Schema & Migrations             |
| 04   | `04-storage-node.md`          | Storage Node gRPC Server                   |
| 05   | `05-coordinator-core.md`      | Coordinator Core: Chunking, RS, Checksums  |
| 06   | `06-coordinator-rest-api.md`  | Coordinator REST API (Upload / Download)   |
| 07   | `07-node-manager.md`          | Node Manager & Health / Simulation         |
| 08   | `08-download-reconstruct.md`  | Download Flow, Reconstruction & Validation |
| 09   | `09-docker-compose.md`        | Docker Compose & Dockerfiles               |
| 10   | `10-react-dashboard.md`       | React + Tailwind Dashboard                 |
| 11   | `11-integration-testing.md`   | End-to-End Integration Testing             |
| 12   | `12-final-polish.md`          | Final Polish, Error Handling & README      |

---

## Key Design Decisions

1. **PostgreSQL stores metadata ONLY** — actual shard bytes live on each node's local disk.
2. **No automatic recovery** — user manually downloads; system reconstructs on-demand.
3. **Node stop/start is simulated** — dashboard calls coordinator REST → coordinator marks node offline and refuses gRPC calls to it.
4. **Failsafe download** — always validate SHA-256 of reconstructed file before sending to user.
5. **gRPC streaming** — use client-streaming for upload and server-streaming for download of large shards.

---

## Execution Order

Read and execute each step file (`01` through `12`) in order. Each file is self-contained with:
- **Goal** — what this step accomplishes
- **Files to create/modify** — exact file paths
- **Code** — complete, working code
- **Verification** — how to confirm the step works

After reviewing each step file, tell me to proceed and I'll implement it.
