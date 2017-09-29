package util

import (
	"io"
	"sync"
)

// SyncedWriter is a io.Writer with a mutex to guard writing from multiple threads
type SyncedWriter struct {
	writer io.Writer
	mutex  sync.Mutex
}

// NewSyncedWriter returns a SyncedWriter which wraps the given io.Writer
func NewSyncedWriter(writer io.Writer) *SyncedWriter {
	return &SyncedWriter{
		writer: writer,
	}
}

// Write implements io.Writer for SyncedWriter
func (w *SyncedWriter) Write(p []byte) (n int, err error) {
	w.mutex.Lock()
	defer w.mutex.Unlock()
	return w.writer.Write(p)
}
