package plugin

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// logWriter is a thread-safe stderr writer for emitting warnings and diagnostics
// during code generation. protoc captures stderr and displays it to the user.
var (
	logMu sync.Mutex
)

// Warn logs a warning message to stderr. It is safe for concurrent use.
// Warnings are prefixed with the plugin name so users can identify the source.
func Warn(format string, args ...interface{}) {
	logMu.Lock()
	defer logMu.Unlock()
	prefix := fmt.Sprintf("[%s] WARN:", filepath.Base(os.Args[0]))
	_, _ = fmt.Fprintf(os.Stderr, prefix+" "+format+"\n", args...)
}
