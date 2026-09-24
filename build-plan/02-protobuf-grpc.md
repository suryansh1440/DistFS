# Step 02 — Protobuf Definitions & gRPC Code Generation

## Goal
Define the protobuf service contract between the Go Coordinator and the 4 Storage Nodes, generate Go code **once** into a shared module, and have both sides import from the same package.

---

## Why This Step First?
Both the coordinator (gRPC client) and storage node (gRPC server) depend on the generated protobuf code. Defining the contract early means both sides can compile against the same interface.

---

## Why a Shared Proto Module?

Instead of generating duplicate code into both `coordinator/` and `storage-node/`, we generate **once** into `proto/gen/` which is its own Go module. Both modules import it.

```
proto/
├── go.mod                    ← its own Go module
├── storage.proto             ← source of truth
└── gen/
    └── storagepb/
        ├── storage.pb.go         ← generated once
        └── storage_grpc.pb.go    ← generated once
```

Benefits:
- **No code drift** — both modules always use the exact same generated types
- **Single source of truth** — one `.proto`, one set of generated files
- **Cleaner imports** — `import pb "github.com/surya/dist-file-storage/proto/gen/storagepb"`

---

## Files to Create

### `proto/storage.proto`

```protobuf
syntax = "proto3";

package storage;

option go_package = "github.com/surya/dist-file-storage/proto/gen/storagepb";

// StorageNode service — runs on each of the 4 storage nodes.
// The coordinator calls these RPCs to store/retrieve/delete shards.
service StorageNode {

    // Store a shard on this node.
    // Uses client-streaming so large shards are sent in small gRPC chunks.
    rpc StoreShard(stream StoreShardRequest) returns (StoreShardResponse);

    // Retrieve a shard from this node.
    // Uses server-streaming so large shards are returned in small gRPC chunks.
    rpc RetrieveShard(RetrieveShardRequest) returns (stream RetrieveShardResponse);

    // Delete a specific shard from this node.
    rpc DeleteShard(DeleteShardRequest) returns (DeleteShardResponse);

    // Delete all shards belonging to a file.
    rpc DeleteFileShards(DeleteFileShardsRequest) returns (DeleteFileShardsResponse);

    // Health check — coordinator pings each node.
    rpc HealthCheck(HealthCheckRequest) returns (HealthCheckResponse);
}

// ─── StoreShard ────────────────────────────────────────────

message StoreShardRequest {
    oneof payload {
        ShardMetadata metadata = 1;   // Sent first: tells the node what this shard is
        bytes data = 2;                // Subsequent messages: raw shard bytes (64KB chunks)
    }
}

message ShardMetadata {
    string file_id = 1;
    string chunk_id = 2;
    int32 shard_index = 3;            // 0..3 (data shards 0-2, parity shard 3)
    string shard_type = 4;            // "DATA" or "PARITY"
    string checksum = 5;              // SHA-256 of the complete shard
    int64 size = 6;                   // Total shard size in bytes
}

message StoreShardResponse {
    bool success = 1;
    string message = 2;
    string shard_id = 3;              // Node-side ID for stored shard
}

// ─── RetrieveShard ─────────────────────────────────────────

message RetrieveShardRequest {
    string file_id = 1;
    string chunk_id = 2;
    int32 shard_index = 3;
}

message RetrieveShardResponse {
    bytes data = 1;                    // 64KB chunks of the shard
}

// ─── DeleteShard ───────────────────────────────────────────

message DeleteShardRequest {
    string file_id = 1;
    string chunk_id = 2;
    int32 shard_index = 3;
}

message DeleteShardResponse {
    bool success = 1;
    string message = 2;
}

// ─── DeleteFileShards ──────────────────────────────────────

message DeleteFileShardsRequest {
    string file_id = 1;
}

message DeleteFileShardsResponse {
    bool success = 1;
    string message = 2;
    int32 shards_deleted = 3;
}

// ─── HealthCheck ───────────────────────────────────────────

message HealthCheckRequest {}

message HealthCheckResponse {
    bool healthy = 1;
    string node_id = 2;
    int64 total_storage = 3;          // bytes
    int64 used_storage = 4;           // bytes
    int64 shard_count = 5;
}
```

### `proto/go.mod`

```
module github.com/surya/dist-file-storage/proto

go 1.22

require (
    google.golang.org/grpc v1.65.0
    google.golang.org/protobuf v1.34.2
)
```

---

## Code Generation

### Generate Go code ONCE into the shared proto module

From project root:

```bash
# Create output directory
mkdir -p proto/gen/storagepb

# Generate into proto/gen/storagepb
protoc --go_out=. --go-grpc_out=. \
    --go_opt=paths=source_relative \
    --go-grpc_opt=paths=source_relative \
    proto/storage.proto
```

> **Note:** Because `go_package` is set to `github.com/surya/dist-file-storage/proto/gen/storagepb`,
> protoc will place the generated files into `proto/gen/storagepb/` relative to the output root.

This creates:

```
proto/gen/storagepb/
    storage.pb.go         # Message types
    storage_grpc.pb.go    # gRPC service interface + client/server stubs
```

---

## Wiring Both Modules to the Shared Proto

Both `coordinator` and `storage-node` use a `replace` directive to point to the local `proto/` module:

### `coordinator/go.mod` — add:

```
require github.com/surya/dist-file-storage/proto v0.0.0

replace github.com/surya/dist-file-storage/proto => ../proto
```

### `storage-node/go.mod` — add:

```
require github.com/surya/dist-file-storage/proto v0.0.0

replace github.com/surya/dist-file-storage/proto => ../proto
```

### Import path in Go code (same for both modules):

```go
import pb "github.com/surya/dist-file-storage/proto/gen/storagepb"
```

---

## Generated Interface Summary

After generation, the storage node will need to implement:

```go
type StorageNodeServer interface {
    StoreShard(StorageNode_StoreShardServer) error
    RetrieveShard(*RetrieveShardRequest, StorageNode_RetrieveShardServer) error
    DeleteShard(context.Context, *DeleteShardRequest) (*DeleteShardResponse, error)
    DeleteFileShards(context.Context, *DeleteFileShardsRequest) (*DeleteFileShardsResponse, error)
    HealthCheck(context.Context, *HealthCheckRequest) (*HealthCheckResponse, error)
}
```

And the coordinator will use the generated client:

```go
type StorageNodeClient interface {
    StoreShard(ctx context.Context, opts ...grpc.CallOption) (StorageNode_StoreShardClient, error)
    RetrieveShard(ctx context.Context, in *RetrieveShardRequest, opts ...grpc.CallOption) (StorageNode_RetrieveShardClient, error)
    DeleteShard(ctx context.Context, in *DeleteShardRequest, opts ...grpc.CallOption) (*DeleteShardResponse, error)
    DeleteFileShards(ctx context.Context, in *DeleteFileShardsRequest, opts ...grpc.CallOption) (*DeleteFileShardsResponse, error)
    HealthCheck(ctx context.Context, in *HealthCheckRequest, opts ...grpc.CallOption) (*HealthCheckResponse, error)
}
```

---

## Design Decisions

| Decision | Rationale |
|----------|-----------|
| **Client-streaming for StoreShard** | Shards can be several MB; streaming avoids loading entire shard in single gRPC message (4MB default limit) |
| **Server-streaming for RetrieveShard** | Same reason — stream shard data in 64KB chunks |
| **Unary for Delete/Health** | Small payloads; no need for streaming |
| **`oneof` in StoreShardRequest** | First message carries metadata, subsequent messages carry raw bytes — clean protocol |
| **Checksum in metadata** | Node can verify shard integrity upon receipt |
| **Shared proto module** | Single source of truth — both coordinator and storage-node import the exact same generated code, preventing drift |

---

## Verification

```bash
# Generated files should exist
ls proto/gen/storagepb/storage.pb.go
ls proto/gen/storagepb/storage_grpc.pb.go

# Compile the proto module
cd proto && go build ./...

# Compile both consuming modules
cd ../coordinator && go build ./...
cd ../storage-node && go build ./...
```

---

## What This Step Achieves
- Formal gRPC contract between coordinator and storage nodes
- **Single shared proto module** — no duplicated generated code
- Generated Go client (coordinator) and server (storage node) stubs
- Streaming protocol for large shard transfers
- Ready to implement the storage node (Step 04) and coordinator (Step 05)
