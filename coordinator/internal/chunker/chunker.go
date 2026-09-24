// Package chunker splits files into fixed-size chunks and merges them back.
//
// Upload:   file → SplitData() → []ChunkResult (each ~4MB)
// Download: []ChunkResult → MergeChunks() → original file bytes
package chunker

import (
	"fmt"
	"math"
)

// DefaultChunkSize is 4 MB — each chunk is independently Reed-Solomon encoded.
const DefaultChunkSize = 4 * 1024 * 1024

// ChunkResult holds one chunk's data and its position within the file.
type ChunkResult struct {
	Index int    // 0-based position in the file
	Data  []byte // raw chunk bytes
	Size  int64  // len(Data) — last chunk may be smaller than DefaultChunkSize
}

// SplitData divides raw file bytes into fixed-size chunks.
// The last chunk may be smaller than chunkSize.
//
// Example: 10MB file with 4MB chunks → [4MB, 4MB, 2MB]
func SplitData(data []byte, chunkSize int) ([]ChunkResult, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("cannot chunk empty data")
	}
	if chunkSize <= 0 {
		return nil, fmt.Errorf("chunk size must be positive, got %d", chunkSize)
	}

	totalChunks := int(math.Ceil(float64(len(data)) / float64(chunkSize)))
	chunks := make([]ChunkResult, 0, totalChunks)

	for i := 0; i < len(data); i += chunkSize {
		end := i + chunkSize
		if end > len(data) {
			end = len(data)
		}

		// Copy to avoid referencing the original slice (safe for concurrent use)
		chunk := make([]byte, end-i)
		copy(chunk, data[i:end])

		chunks = append(chunks, ChunkResult{
			Index: len(chunks),
			Data:  chunk,
			Size:  int64(len(chunk)),
		})
	}

	return chunks, nil
}

// MergeChunks reassembles ordered chunks back into the original file data.
// Chunks MUST be sorted by Index before calling this.
func MergeChunks(chunks []ChunkResult) []byte {
	totalSize := int64(0)
	for _, c := range chunks {
		totalSize += c.Size
	}

	result := make([]byte, 0, totalSize)
	for _, c := range chunks {
		result = append(result, c.Data...)
	}

	return result
}

// CalculateTotalChunks returns how many chunks a file of the given size produces.
func CalculateTotalChunks(fileSize int64, chunkSize int) int {
	return int(math.Ceil(float64(fileSize) / float64(chunkSize)))
}
