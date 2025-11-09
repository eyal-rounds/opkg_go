//go:build debug

package logging

import (
	"fmt"
	"os"
	"sync"
	"time"
)

var (
	mu sync.Mutex
)

// Debugf prints formatted debug output when the debug build tag is enabled.
func Debugf(format string, args ...interface{}) {
	mu.Lock()
	defer mu.Unlock()
	timestamp := time.Now().Format(time.RFC3339)
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(os.Stderr, "[%s][DEBUG] %s\n", timestamp, msg)
}
