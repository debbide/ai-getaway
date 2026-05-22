package service

import (
	"bytes"
	"io"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

const runtimeLogLimit = 800

type RuntimeLogEntry struct {
	Time    time.Time `json:"time"`
	Message string    `json:"message"`
}

type runtimeLogBuffer struct {
	mu      sync.RWMutex
	entries []RuntimeLogEntry
	pending []byte
}

var runtimeLogs = &runtimeLogBuffer{}

func InstallRuntimeLogCapture() {
	writer := io.MultiWriter(os.Stderr, runtimeLogs)
	log.SetOutput(writer)
	gin.DefaultWriter = io.MultiWriter(os.Stdout, runtimeLogs)
	gin.DefaultErrorWriter = writer
}

func RuntimeLogs(limit int) []RuntimeLogEntry {
	return runtimeLogs.List(limit)
}

func (b *runtimeLogBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	data := append(b.pending, p...)
	lines := bytes.Split(data, []byte{'\n'})
	for _, line := range lines[:len(lines)-1] {
		b.appendLocked(string(line))
	}
	b.pending = append(b.pending[:0], lines[len(lines)-1]...)
	return len(p), nil
}

func (b *runtimeLogBuffer) List(limit int) []RuntimeLogEntry {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if limit <= 0 || limit > runtimeLogLimit {
		limit = 200
	}
	start := len(b.entries) - limit
	if start < 0 {
		start = 0
	}
	items := make([]RuntimeLogEntry, len(b.entries[start:]))
	copy(items, b.entries[start:])
	return items
}

func (b *runtimeLogBuffer) appendLocked(message string) {
	message = strings.TrimSpace(message)
	if message == "" {
		return
	}
	b.entries = append(b.entries, RuntimeLogEntry{Time: time.Now(), Message: message})
	if len(b.entries) > runtimeLogLimit {
		copy(b.entries, b.entries[len(b.entries)-runtimeLogLimit:])
		b.entries = b.entries[:runtimeLogLimit]
	}
}
