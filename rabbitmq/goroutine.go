package rabbitmq

import (
	"bytes"
	"runtime"
	"strconv"
)

// getGoroutineID returns the ID of the current goroutine
// This is used for per-goroutine channel caching
func getGoroutineID() uint64 {
	b := make([]byte, 64)
	b = b[:runtime.Stack(b, false)]
	// Parse "goroutine 123 ["
	b = bytes.TrimPrefix(b, []byte("goroutine "))
	b = b[:bytes.IndexByte(b, ' ')]
	n, _ := strconv.ParseUint(string(b), 10, 64)
	return n
}
