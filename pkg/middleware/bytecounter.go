package middleware

import (
	"net/http"
)

// ByteCounterWriter is a wrapper around http.ResponseWriter that counts bytes written
type ByteCounterWriter struct {
	http.ResponseWriter
	BytesWritten int64
	StatusCode   int
}

// NewByteCounterWriter creates a new ByteCounterWriter
func NewByteCounterWriter(w http.ResponseWriter) *ByteCounterWriter {
	return &ByteCounterWriter{
		ResponseWriter: w,
		StatusCode:     http.StatusOK, // Default status code
	}
}

// Write counts bytes written and passes to the underlying ResponseWriter
func (bcw *ByteCounterWriter) Write(b []byte) (int, error) {
	n, err := bcw.ResponseWriter.Write(b)
	bcw.BytesWritten += int64(n)
	return n, err
}

// WriteHeader captures the status code and passes to the underlying ResponseWriter
func (bcw *ByteCounterWriter) WriteHeader(statusCode int) {
	bcw.StatusCode = statusCode
	bcw.ResponseWriter.WriteHeader(statusCode)
}

// Flush implements http.Flusher if the underlying ResponseWriter supports it
func (bcw *ByteCounterWriter) Flush() {
	if flusher, ok := bcw.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

// Hijack implements http.Hijacker if the underlying ResponseWriter supports it
func (bcw *ByteCounterWriter) Hijack() (interface{}, interface{}, error) {
	if hijacker, ok := bcw.ResponseWriter.(http.Hijacker); ok {
		return hijacker.Hijack()
	}
	return nil, nil, http.ErrNotSupported
}
