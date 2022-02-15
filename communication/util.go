package communication

import "math"

const (
	ChunkSize int64 = 1024 * 1024
)

func CalculateNumberOfChunks(fileSize int64) int {
	return int(math.Ceil(float64(fileSize) / float64(ChunkSize)))
}
