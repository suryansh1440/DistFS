# Step 09 — Docker Compose & Dockerfiles

## Goal
Create all Dockerfiles and a Docker Compose configuration to run the complete system: PostgreSQL + Go Coordinator + 4 Storage Nodes + React Dashboard.

---

## Complete Docker Compose Architecture

```
docker-compose.yml
    │
    ├── postgres          (PostgreSQL 16)       port 5432
    ├── coordinator        (Go REST API)         port 8080
    ├── storage-node-1     (Go gRPC)             port 50051
    ├── storage-node-2     (Go gRPC)             port 50052
    ├── storage-node-3     (Go gRPC)             port 50053
    ├── storage-node-4     (Go gRPC)             port 50054
    └── dashboard          (React + Vite)        port 5173
```

---

## Files to Create

### `Dockerfile.coordinator`

```dockerfile
# Build stage
FROM golang:1.22-alpine AS builder

WORKDIR /app

# Copy go.mod and go.sum first for layer caching
COPY coordinator/go.mod coordinator/go.sum ./
RUN go mod download

# Copy source code
COPY coordinator/ .
COPY migrations/ /migrations/

# Build
RUN CGO_ENABLED=0 GOOS=linux go build -o /coordinator ./cmd/server

# Run stage
FROM alpine:3.19

RUN apk --no-cache add ca-certificates

WORKDIR /app

COPY --from=builder /coordinator .
COPY --from=builder /migrations/ ./migrations/

EXPOSE 8080

CMD ["./coordinator"]
```

### `Dockerfile.storage-node`

```dockerfile
# Build stage
FROM golang:1.22-alpine AS builder

WORKDIR /app

COPY storage-node/go.mod storage-node/go.sum ./
RUN go mod download

COPY storage-node/ .

RUN CGO_ENABLED=0 GOOS=linux go build -o /storage-node ./cmd/node

# Run stage
FROM alpine:3.19

RUN apk --no-cache add ca-certificates

WORKDIR /app

COPY --from=builder /storage-node .

# Create data directory
RUN mkdir -p /data/shards

EXPOSE 50051

CMD ["./storage-node"]
```

### `Dockerfile.dashboard`

```dockerfile
FROM node:20-alpine

WORKDIR /app

COPY dashboard/package*.json ./
RUN npm install

COPY dashboard/ .

EXPOSE 5173

CMD ["npm", "run", "dev", "--", "--host", "0.0.0.0"]
```

### `docker-compose.yml`

```yaml
version: "3.8"

services:
  # ─── PostgreSQL ──────────────────────────────────────
  postgres:
    image: postgres:16-alpine
    container_name: distfs-postgres
    environment:
      POSTGRES_USER: distfs
      POSTGRES_PASSWORD: distfs_secret
      POSTGRES_DB: distfs
    ports:
      - "5432:5432"
    volumes:
      - postgres_data:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U distfs"]
      interval: 5s
      timeout: 5s
      retries: 5
    networks:
      - distfs-network

  # ─── Storage Node 1 ─────────────────────────────────
  storage-node-1:
    build:
      context: .
      dockerfile: Dockerfile.storage-node
    container_name: distfs-node-1
    environment:
      NODE_ID: node-1
      GRPC_PORT: "50051"
      DATA_DIR: /data/shards
    ports:
      - "50051:50051"
    volumes:
      - node1_data:/data/shards
    networks:
      - distfs-network

  # ─── Storage Node 2 ─────────────────────────────────
  storage-node-2:
    build:
      context: .
      dockerfile: Dockerfile.storage-node
    container_name: distfs-node-2
    environment:
      NODE_ID: node-2
      GRPC_PORT: "50052"
      DATA_DIR: /data/shards
    ports:
      - "50052:50052"
    volumes:
      - node2_data:/data/shards
    networks:
      - distfs-network

  # ─── Storage Node 3 ─────────────────────────────────
  storage-node-3:
    build:
      context: .
      dockerfile: Dockerfile.storage-node
    container_name: distfs-node-3
    environment:
      NODE_ID: node-3
      GRPC_PORT: "50053"
      DATA_DIR: /data/shards
    ports:
      - "50053:50053"
    volumes:
      - node3_data:/data/shards
    networks:
      - distfs-network

  # ─── Storage Node 4 ─────────────────────────────────
  storage-node-4:
    build:
      context: .
      dockerfile: Dockerfile.storage-node
    container_name: distfs-node-4
    environment:
      NODE_ID: node-4
      GRPC_PORT: "50054"
      DATA_DIR: /data/shards
    ports:
      - "50054:50054"
    volumes:
      - node4_data:/data/shards
    networks:
      - distfs-network

  # ─── Go Coordinator ─────────────────────────────────
  coordinator:
    build:
      context: .
      dockerfile: Dockerfile.coordinator
    container_name: distfs-coordinator
    environment:
      POSTGRES_HOST: postgres
      POSTGRES_PORT: "5432"
      POSTGRES_USER: distfs
      POSTGRES_PASSWORD: distfs_secret
      POSTGRES_DB: distfs
      COORDINATOR_PORT: "8080"
      CHUNK_SIZE: "4194304"
      DATA_SHARDS: "3"
      PARITY_SHARDS: "1"
    ports:
      - "8080:8080"
    depends_on:
      postgres:
        condition: service_healthy
      storage-node-1:
        condition: service_started
      storage-node-2:
        condition: service_started
      storage-node-3:
        condition: service_started
      storage-node-4:
        condition: service_started
    networks:
      - distfs-network

  # ─── React Dashboard ────────────────────────────────
  dashboard:
    build:
      context: .
      dockerfile: Dockerfile.dashboard
    container_name: distfs-dashboard
    ports:
      - "5173:5173"
    depends_on:
      - coordinator
    networks:
      - distfs-network

volumes:
  postgres_data:
  node1_data:
  node2_data:
  node3_data:
  node4_data:

networks:
  distfs-network:
    driver: bridge
```

---

## Important Notes on Port Mapping

Each storage node listens on a **different port** even though the Dockerfile exposes 50051:

| Container | Container Port | Host Port | GRPC_PORT env |
|-----------|:---:|:---:|:---:|
| storage-node-1 | 50051 | 50051 | 50051 |
| storage-node-2 | 50052 | 50052 | 50052 |
| storage-node-3 | 50053 | 50053 | 50053 |
| storage-node-4 | 50054 | 50054 | 50054 |

The **coordinator** connects to nodes using Docker Compose service names:
- `storage-node-1:50051`
- `storage-node-2:50052`
- `storage-node-3:50053`
- `storage-node-4:50054`

These names resolve inside the `distfs-network` Docker network.

---

## Startup Order

```
postgres (must be healthy first)
    │
    ▼
storage-node-1, storage-node-2, storage-node-3, storage-node-4
    │
    ▼
coordinator (depends on postgres + all storage nodes)
    │
    ▼
dashboard (depends on coordinator)
```

---

## `.env` file (project root)

```env
# PostgreSQL
POSTGRES_HOST=postgres
POSTGRES_PORT=5432
POSTGRES_USER=distfs
POSTGRES_PASSWORD=distfs_secret
POSTGRES_DB=distfs

# Coordinator
COORDINATOR_PORT=8080
CHUNK_SIZE=4194304

# Reed-Solomon
DATA_SHARDS=3
PARITY_SHARDS=1
```

---

## Commands

```bash
# Build all images
docker-compose build

# Start everything
docker-compose up -d

# View logs
docker-compose logs -f coordinator
docker-compose logs -f storage-node-1

# Check all containers are running
docker-compose ps

# Stop everything
docker-compose down

# Stop and remove volumes (clean reset)
docker-compose down -v
```

---

## Network Diagram

```
┌───────────────────── distfs-network ──────────────────────┐
│                                                           │
│  ┌──────────┐     ┌───────────────┐     ┌──────────────┐ │
│  │ postgres │◄────│  coordinator  │────►│ storage-node │ │
│  │  :5432   │     │    :8080      │     │  -1  :50051  │ │
│  └──────────┘     └───────┬───────┘     └──────────────┘ │
│                           │              ┌──────────────┐ │
│  ┌──────────┐             │             │ storage-node │ │
│  │dashboard │             ├────────────►│  -2  :50052  │ │
│  │  :5173   │             │             └──────────────┘ │
│  └──────────┘             │              ┌──────────────┐ │
│                           ├────────────►│ storage-node │ │
│                           │             │  -3  :50053  │ │
│                           │             └──────────────┘ │
│                           │              ┌──────────────┐ │
│                           └────────────►│ storage-node │ │
│                                         │  -4  :50054  │ │
│                                         └──────────────┘ │
└───────────────────────────────────────────────────────────┘
         │              │
    Host:5173      Host:8080
    (Dashboard)    (REST API)
```

---

## Verification

```bash
# Build and start
docker-compose up --build -d

# Wait for healthy status
docker-compose ps

# All 7 containers should be running:
# distfs-postgres         running (healthy)
# distfs-node-1           running
# distfs-node-2           running
# distfs-node-3           running
# distfs-node-4           running
# distfs-coordinator      running
# distfs-dashboard        running

# Test coordinator health
curl http://localhost:8080/api/health
# → {"status":"healthy","service":"coordinator"}

# Test node listing
curl http://localhost:8080/api/nodes
# → Array of 4 nodes, all ONLINE

# View coordinator logs
docker-compose logs coordinator
# Should show: migrations applied, nodes connected
```

---

## What This Step Achieves
- Complete Docker Compose setup with 7 services
- Multi-stage Dockerfiles (small final images)
- Named volumes for data persistence
- Health checks for PostgreSQL
- Proper startup ordering
- Internal Docker network for service-to-service communication
- Ready to test end-to-end upload/download/reconstruction
