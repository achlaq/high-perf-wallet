package logger

import (
	"context"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var Log *zap.Logger

// contextKey defines a custom type for context keys to avoid package collisions
type contextKey string

// RequestIDKey is the context key where the HTTP Request ID is stored
const RequestIDKey contextKey = "request_id"

// InitLogger initializes the global production logger
func InitLogger() {
	config := zap.NewProductionConfig()
	config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	var err error
	Log, err = config.Build()
	if err != nil {
		panic(err)
	}
}

// WithCtx returns a logger contextualized with the request ID if it exists in the context
func WithCtx(ctx context.Context) *zap.Logger {
	if ctx == nil {
		return Log
	}
	if reqID, ok := ctx.Value(RequestIDKey).(string); ok {
		return Log.With(zap.String("request_id", reqID))
	}
	return Log
}
