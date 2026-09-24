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
	dataDir := getEnv("DATA_DIR", "./data/shards")
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
		grpc.MaxRecvMsgSize(8*1024*1024), // 8MB max message
		grpc.MaxSendMsgSize(8*1024*1024),
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

	log.Printf("[OK] Storage node %s listening on :%s", nodeID, port)
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
