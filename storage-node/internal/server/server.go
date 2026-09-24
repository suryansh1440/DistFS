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
	nodeID       string
	diskStore    *store.DiskStore
	totalStorage int64 // configured total capacity in bytes
}

// NewStorageServer creates a new StorageServer instance.
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
	log.Printf("[%s] [OK] Stored shard: %s (%d bytes)", s.nodeID, shardID, buf.Len())

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

	log.Printf("[%s] [OK] Sent shard: %d bytes", s.nodeID, len(data))
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
