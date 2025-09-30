package middleware

// Logger interface that users can implement
type Logger interface {
	Info(msg string, keysAndValues ...interface{})
	Error(msg string, keysAndValues ...interface{})
}

// NoOpLogger is a logger that discards all logs
type NoOpLogger struct{}

func (n NoOpLogger) Info(msg string, keysAndValues ...interface{})  {}
func (n NoOpLogger) Error(msg string, keysAndValues ...interface{}) {}
