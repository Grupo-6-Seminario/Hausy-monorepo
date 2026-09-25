// Package logging carries a request-scoped logger across backend modules.
package logging

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"syscall"
)

type loggerKey struct{}

// WithLogger attaches a logger to a request without changing application interfaces.
func WithLogger(ctx context.Context, logger *slog.Logger) context.Context {
	return context.WithValue(ctx, loggerKey{}, logger)
}

// FromContext returns the request logger, or the configured process logger.
func FromContext(ctx context.Context) *slog.Logger {
	if logger, ok := ctx.Value(loggerKey{}).(*slog.Logger); ok && logger != nil {
		return logger
	}
	return slog.Default()
}

// ErrorClass keeps diagnostics useful without serializing errors that may
// contain prompts, provider bodies, credentials, or database connection URIs.
func ErrorClass(err error) string {
	switch {
	case errors.Is(err, context.Canceled):
		return "canceled"
	case errors.Is(err, context.DeadlineExceeded):
		return "deadline"
	case errors.Is(err, syscall.EADDRINUSE):
		return "address_in_use"
	case errors.Is(err, syscall.EACCES):
		return "permission_denied"
	case errors.Is(err, syscall.ECONNREFUSED):
		return "connection_refused"
	}
	var network net.Error
	if errors.As(err, &network) && network.Timeout() {
		return "timeout"
	}
	return "failed"
}
