package shardmanager

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/surya/dist-file-storage/coordinator/internal/checksum"
	"github.com/surya/dist-file-storage/coordinator/internal/db"
	pb "github.com/surya/dist-file-storage/proto/gen/storagepb"
)

const grpcChunkSize = 64 * 1024 // 64KB per stream message

// ShardManager handles distributing and retrieving shards from storage nodes.
type ShardManager struct {
	connections map[string]pb.StorageNodeClient // nodeID -> gRPC client
	mu          sync.RWMutex
}

func NewShardManager() *ShardManager {
	return &ShardManager{
		connections: make(map[string]pb.StorageNodeClient),
	}
}

// ConnectToNodes establishes gRPC connections to all nodes.
func (sm *ShardManager) ConnectToNodes() error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	nodes, err := db.ListNodes()
	if err != nil {
		return fmt.Errorf("failed to list nodes: %w", err)
	}

	for _, node := range nodes {
		conn, err := grpc.Dial(
			node.GRPCAddress,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
			grpc.WithDefaultCallOptions(
				grpc.MaxCallRecvMsgSize(8*1024*1024),
				grpc.MaxCallSendMsgSize(8*1024*1024),
			),
		)
		if err != nil {
			log.Printf("[WARN] Failed to connect to node %s (%s): %v", node.ID, node.GRPCAddress, err)
			continue
		}

		sm.connections[node.ID] = pb.NewStorageNodeClient(conn)
		log.Printf("[OK] Connected to node %s at %s", node.ID, node.GRPCAddress)
	}

	return nil
}

// StoreShard sends a shard to the designated storage node via gRPC client-streaming.
func (sm *ShardManager) StoreShard(nodeID, fileID, chunkID string, shardIndex int, shardType string, data []byte) error {
	sm.mu.RLock()
	client, ok := sm.connections[nodeID]
	sm.mu.RUnlock()

	if !ok {
		return fmt.Errorf("no connection to node %s", nodeID)
	}

	// Check if node is online
	node, err := db.GetNode(nodeID)
	if err != nil {
		return err
	}
	if node.Status != "ONLINE" {
		return fmt.Errorf("node %s is OFFLINE", nodeID)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	stream, err := client.StoreShard(ctx)
	if err != nil {
		return fmt.Errorf("failed to open store stream to %s: %w", nodeID, err)
	}

	// Send metadata first
	shardChecksum := checksum.CalculateFromBytes(data)
	err = stream.Send(&pb.StoreShardRequest{
		Payload: &pb.StoreShardRequest_Metadata{
			Metadata: &pb.ShardMetadata{
				FileId:     fileID,
				ChunkId:    chunkID,
				ShardIndex: int32(shardIndex),
				ShardType:  shardType,
				Checksum:   shardChecksum,
				Size:       int64(len(data)),
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to send metadata: %w", err)
	}

	// Stream data in 64KB chunks
	for offset := 0; offset < len(data); offset += grpcChunkSize {
		end := offset + grpcChunkSize
		if end > len(data) {
			end = len(data)
		}

		err = stream.Send(&pb.StoreShardRequest{
			Payload: &pb.StoreShardRequest_Data{
				Data: data[offset:end],
			},
		})
		if err != nil {
			return fmt.Errorf("failed to send data chunk: %w", err)
		}
	}

	resp, err := stream.CloseAndRecv()
	if err != nil {
		return fmt.Errorf("failed to close stream: %w", err)
	}

	if !resp.Success {
		return fmt.Errorf("node rejected shard: %s", resp.Message)
	}

	log.Printf("[OK] Stored shard %d on %s (checksum: %s...)", shardIndex, nodeID, shardChecksum[:8])
	return nil
}

// RetrieveShard retrieves a shard from a storage node via gRPC server-streaming.
func (sm *ShardManager) RetrieveShard(nodeID, fileID, chunkID string, shardIndex int) ([]byte, error) {
	sm.mu.RLock()
	client, ok := sm.connections[nodeID]
	sm.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("no connection to node %s", nodeID)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	stream, err := client.RetrieveShard(ctx, &pb.RetrieveShardRequest{
		FileId:     fileID,
		ChunkId:    chunkID,
		ShardIndex: int32(shardIndex),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to open retrieve stream from %s: %w", nodeID, err)
	}

	var buf bytes.Buffer
	for {
		resp, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to receive data from %s: %w", nodeID, err)
		}
		buf.Write(resp.Data)
	}

	return buf.Bytes(), nil
}

// DeleteFileFromNode deletes all shards for a file from a specific node.
func (sm *ShardManager) DeleteFileFromNode(nodeID, fileID string) error {
	sm.mu.RLock()
	client, ok := sm.connections[nodeID]
	sm.mu.RUnlock()

	if !ok {
		return fmt.Errorf("no connection to node %s", nodeID)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := client.DeleteFileShards(ctx, &pb.DeleteFileShardsRequest{FileId: fileID})
	if err != nil {
		return fmt.Errorf("failed to delete shards from %s: %w", nodeID, err)
	}

	log.Printf("[OK] Deleted %d shards from %s for file %s", resp.ShardsDeleted, nodeID, fileID)
	return nil
}

// HealthCheckNode pings a specific node and returns health info.
func (sm *ShardManager) HealthCheckNode(nodeID string) (*pb.HealthCheckResponse, error) {
	sm.mu.RLock()
	client, ok := sm.connections[nodeID]
	sm.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("no connection to node %s", nodeID)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := client.HealthCheck(ctx, &pb.HealthCheckRequest{})
	if err != nil {
		return nil, fmt.Errorf("health check failed for %s: %w", nodeID, err)
	}

	return resp, nil
}

// GetNodeAssignment returns which node a shard should go to.
// Simple mapping: shard_index -> node_id
// Shard 0 -> node-1, Shard 1 -> node-2, Shard 2 -> node-3, Shard 3 -> node-4
func GetNodeAssignment(shardIndex int) string {
	nodeIDs := []string{"node-1", "node-2", "node-3", "node-4"}
	return nodeIDs[shardIndex%len(nodeIDs)]
}

// GetShardType returns "DATA" or "PARITY" based on the shard index and config.
func GetShardType(shardIndex, dataShards int) string {
	if shardIndex < dataShards {
		return "DATA"
	}
	return "PARITY"
}
