package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestByteCounterWriter(t *testing.T) {
	tests := []struct {
		name           string
		writeData      []byte
		expectedBytes  int64
		expectedStatus int
	}{
		{
			name:           "empty response",
			writeData:      []byte{},
			expectedBytes:  0,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "small response",
			writeData:      []byte("Hello, World!"),
			expectedBytes:  13,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "larger response",
			writeData:      make([]byte, 1024),
			expectedBytes:  1024,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "custom status code",
			writeData:      []byte("Not Found"),
			expectedBytes:  9,
			expectedStatus: http.StatusNotFound,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Create a test response recorder
			rec := httptest.NewRecorder()

			// Create our byte counter wrapper
			bcw := NewByteCounterWriter(rec)

			// Set status code if needed
			if tc.expectedStatus != http.StatusOK {
				bcw.WriteHeader(tc.expectedStatus)
			}

			// Write data
			n, err := bcw.Write(tc.writeData)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			if n != len(tc.writeData) {
				t.Errorf("Expected to write %d bytes, wrote %d", len(tc.writeData), n)
			}

			// Check bytes counted
			if bcw.BytesWritten != tc.expectedBytes {
				t.Errorf("Expected BytesWritten to be %d, got %d", tc.expectedBytes, bcw.BytesWritten)
			}

			// Check status code
			if bcw.StatusCode != tc.expectedStatus {
				t.Errorf("Expected StatusCode to be %d, got %d", tc.expectedStatus, bcw.StatusCode)
			}
		})
	}
}

func TestByteCounterWriterMultipleWrites(t *testing.T) {
	rec := httptest.NewRecorder()
	bcw := NewByteCounterWriter(rec)

	// Write multiple times
	bcw.Write([]byte("Hello"))
	bcw.Write([]byte(", "))
	bcw.Write([]byte("World!"))

	expectedBytes := int64(13) // "Hello, World!"
	if bcw.BytesWritten != expectedBytes {
		t.Errorf("Expected BytesWritten to be %d, got %d", expectedBytes, bcw.BytesWritten)
	}

	// Check the actual content
	if rec.Body.String() != "Hello, World!" {
		t.Errorf("Expected body to be 'Hello, World!', got '%s'", rec.Body.String())
	}
}
