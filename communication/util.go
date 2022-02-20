package communication

import "math"

// CalculateNumberOfChunks calculates the number of chunks a file has
func CalculateNumberOfChunks(fileSize int64) int {
	return int(math.Ceil(float64(fileSize) / float64(ChunkSize)))
}
