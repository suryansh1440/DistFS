# Step 03 — PostgreSQL Schema & Migrations

## Goal
Design and create the PostgreSQL database schema that stores all metadata for files, chunks, shards, and nodes. No actual file/shard binary data goes here.

---

## Database: `distfs`

---

## Migration Files

### `migrations/001_create_nodes.sql`

```sql
-- Represents each of the 4 storage nodes in the cluster.
-- Status is managed by the coordinator (simulation dashboard can stop/start).

CREATE TABLE IF NOT EXISTS nodes (
    id              VARCHAR(36) PRIMARY KEY,          -- UUID
    name            VARCHAR(50) NOT NULL UNIQUE,      -- e.g., "node-1"
    grpc_address    VARCHAR(255) NOT NULL,             -- e.g., "storage-node-1:50051"
    status          VARCHAR(20) NOT NULL DEFAULT 'ONLINE',  -- ONLINE | OFFLINE
    total_storage   BIGINT NOT NULL DEFAULT 0,         -- bytes
    used_storage    BIGINT NOT NULL DEFAULT 0,         -- bytes
    shard_count     INT NOT NULL DEFAULT 0,
    last_heartbeat  TIMESTAMP WITH TIME ZONE,
    created_at      TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at      TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Seed the 4 storage nodes
INSERT INTO nodes (id, name, grpc_address, status, total_storage) VALUES
    ('node-1', 'node-1', 'storage-node-1:50051', 'ONLINE', 10737418240),
    ('node-2', 'node-2', 'storage-node-2:50052', 'ONLINE', 10737418240),
    ('node-3', 'node-3', 'storage-node-3:50053', 'ONLINE', 10737418240),
    ('node-4', 'node-4', 'storage-node-4:50054', 'ONLINE', 10737418240)
ON CONFLICT (id) DO NOTHING;
```

### `migrations/002_create_files.sql`

```sql
-- Represents an uploaded file.
-- Stores the ORIGINAL file's SHA-256 checksum for validation during download.

CREATE TABLE IF NOT EXISTS files (
    id                VARCHAR(36) PRIMARY KEY,         -- UUID
    filename          VARCHAR(500) NOT NULL,
    file_size         BIGINT NOT NULL,                  -- original file size in bytes
    original_checksum VARCHAR(64) NOT NULL,             -- SHA-256 hex string
    total_chunks      INT NOT NULL,
    data_shards       INT NOT NULL DEFAULT 3,           -- K
    parity_shards     INT NOT NULL DEFAULT 1,           -- M
    chunk_size        INT NOT NULL DEFAULT 4194304,     -- 4MB
    status            VARCHAR(20) NOT NULL DEFAULT 'UPLOADING',
                      -- UPLOADING | HEALTHY | DEGRADED | FAILED
    created_at        TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at        TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX idx_files_status ON files(status);
CREATE INDEX idx_files_filename ON files(filename);
```

### `migrations/003_create_chunks.sql`

```sql
-- Represents one chunk of a file.
-- Each chunk is Reed-Solomon encoded into (K + M) shards.

CREATE TABLE IF NOT EXISTS chunks (
    id              VARCHAR(36) PRIMARY KEY,            -- UUID
    file_id         VARCHAR(36) NOT NULL REFERENCES files(id) ON DELETE CASCADE,
    chunk_index     INT NOT NULL,                       -- 0-based index within the file
    chunk_size      BIGINT NOT NULL,                    -- actual size of this chunk in bytes
    created_at      TIMESTAMP WITH TIME ZONE DEFAULT NOW(),

    UNIQUE(file_id, chunk_index)
);

CREATE INDEX idx_chunks_file_id ON chunks(file_id);
```

### `migrations/004_create_shards.sql`

```sql
-- Represents one shard (data or parity) of a chunk.
-- Each chunk produces K data shards + M parity shards.
-- The actual shard BYTES are on the storage node's local disk, NOT here.

CREATE TABLE IF NOT EXISTS shards (
    id              VARCHAR(36) PRIMARY KEY,            -- UUID
    chunk_id        VARCHAR(36) NOT NULL REFERENCES chunks(id) ON DELETE CASCADE,
    file_id         VARCHAR(36) NOT NULL REFERENCES files(id) ON DELETE CASCADE,
    shard_index     INT NOT NULL,                       -- 0-based; 0..K-1 = data, K..K+M-1 = parity
    shard_type      VARCHAR(10) NOT NULL,               -- DATA | PARITY
    node_id         VARCHAR(36) NOT NULL REFERENCES nodes(id),
    shard_size      BIGINT NOT NULL,                    -- size in bytes
    checksum        VARCHAR(64) NOT NULL,               -- SHA-256 of this individual shard
    status          VARCHAR(20) NOT NULL DEFAULT 'STORED',
                    -- STORED | MISSING | CORRUPTED
    created_at      TIMESTAMP WITH TIME ZONE DEFAULT NOW(),

    UNIQUE(chunk_id, shard_index)
);

CREATE INDEX idx_shards_chunk_id ON shards(chunk_id);
CREATE INDEX idx_shards_file_id ON shards(file_id);
CREATE INDEX idx_shards_node_id ON shards(node_id);
```

---

## Entity Relationship Diagram

```
┌──────────┐     1:N     ┌──────────┐     1:N     ┌──────────┐
│  files   │────────────►│  chunks  │────────────►│  shards  │
│          │             │          │             │          │
│ id       │             │ id       │             │ id       │
│ filename │             │ file_id  │             │ chunk_id │
│ checksum │             │ chunk_idx│             │ file_id  │
│ status   │             │ size     │             │ shard_idx│
└──────────┘             └──────────┘             │ node_id ─┼──► nodes
                                                  │ type     │
                                                  │ checksum │
                                                  └──────────┘
                                                       │
                                                  N:1  │
                                                       ▼
                                                  ┌──────────┐
                                                  │  nodes   │
                                                  │          │
                                                  │ id       │
                                                  │ name     │
                                                  │ address  │
                                                  │ status   │
                                                  └──────────┘
```

---

## Coordinator Database Connection Code

### `coordinator/internal/db/connection.go`

```go
package db

import (
    "database/sql"
    "fmt"
    "log"
    "os"

    _ "github.com/lib/pq"
)

var DB *sql.DB

func InitDB() error {
    connStr := fmt.Sprintf(
        "host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
        os.Getenv("POSTGRES_HOST"),
        os.Getenv("POSTGRES_PORT"),
        os.Getenv("POSTGRES_USER"),
        os.Getenv("POSTGRES_PASSWORD"),
        os.Getenv("POSTGRES_DB"),
    )

    var err error
    DB, err = sql.Open("postgres", connStr)
    if err != nil {
        return fmt.Errorf("failed to open database: %w", err)
    }

    if err = DB.Ping(); err != nil {
        return fmt.Errorf("failed to ping database: %w", err)
    }

    DB.SetMaxOpenConns(25)
    DB.SetMaxIdleConns(5)

    log.Println("✓ Connected to PostgreSQL")
    return nil
}

func RunMigrations() error {
    migrations := []string{
        "migrations/001_create_nodes.sql",
        "migrations/002_create_files.sql",
        "migrations/003_create_chunks.sql",
        "migrations/004_create_shards.sql",
    }

    for _, file := range migrations {
        content, err := os.ReadFile(file)
        if err != nil {
            return fmt.Errorf("failed to read migration %s: %w", file, err)
        }

        _, err = DB.Exec(string(content))
        if err != nil {
            return fmt.Errorf("failed to execute migration %s: %w", file, err)
        }

        log.Printf("✓ Migration applied: %s", file)
    }

    return nil
}
```

---

## Data Flow Example

When a 100MB file is uploaded:

| Table   | Rows Created |
|---------|-------------|
| `files`  | 1 row (file metadata + SHA-256) |
| `chunks` | 25 rows (100MB / 4MB = 25 chunks) |
| `shards` | 100 rows (25 chunks × 4 shards each) |

When node-2 goes OFFLINE:

| Table   | Change |
|---------|--------|
| `nodes` | `node-2.status = 'OFFLINE'` |
| `shards` | Shards on node-2 → `status = 'MISSING'` (optional, can be computed) |

---

## Verification

```bash
# Start PostgreSQL (Docker for now)
docker run -d --name distfs-postgres \
    -e POSTGRES_USER=distfs \
    -e POSTGRES_PASSWORD=distfs_secret \
    -e POSTGRES_DB=distfs \
    -p 5432:5432 \
    postgres:16

# Connect and verify
psql -h localhost -U distfs -d distfs

# Run migrations manually to test
psql -h localhost -U distfs -d distfs -f migrations/001_create_nodes.sql
psql -h localhost -U distfs -d distfs -f migrations/002_create_files.sql
psql -h localhost -U distfs -d distfs -f migrations/003_create_chunks.sql
psql -h localhost -U distfs -d distfs -f migrations/004_create_shards.sql

# Verify tables
\dt
# Should show: nodes, files, chunks, shards

# Verify seed data
SELECT * FROM nodes;
# Should show 4 nodes, all ONLINE
```

---

## What This Step Achieves
- Complete metadata schema for the distributed storage system
- Clear separation: metadata in PostgreSQL, actual bytes on storage nodes
- Migration files ready for both development and Docker Compose
- Database connection pool configured in Go
