package communication

import "math"

func CalculateNumberOfChunks(fileSize int64) int {
	return int(math.Ceil(float64(fileSize) / float64(ChunkSize)))
}
