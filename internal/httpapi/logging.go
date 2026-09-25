package httpapi

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"time"

	"github.com/Grupo-6-Seminario/proyecto-angus-back/internal/logging"
)

type requestStateKey struct{}

type requestState struct{ outcome string }

func setOutcome(ctx context.Context, outcome string) {
	if state, ok := ctx.Value(requestStateKey{}).(*requestState); ok {
		state.outcome = outcome
	}
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(status int) {
	if w.status != 0 {
		return
	}
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *statusWriter) Write(data []byte) (int, error) {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(data)
}

// Unwrap lets http.ResponseController flush streaming responses.
func (w *statusWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

func logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var random [16]byte
		if _, err := rand.Read(random[:]); err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			slog.Error("request_id_unavailable")
			return
		}
		id := hex.EncodeToString(random[:])
		w.Header().Set("X-Request-ID", id)
		state := &requestState{}
		ctx := context.WithValue(r.Context(), requestStateKey{}, state)
		logger := logging.FromContext(ctx).With("request_id", id)
		ctx = logging.WithLogger(ctx, logger)
		r = r.WithContext(ctx)
		tracked := &statusWriter{ResponseWriter: w}
		started := time.Now()
		next.ServeHTTP(tracked, r)
		status := tracked.status
		if status == 0 {
			status = http.StatusOK
		}
		outcome := state.outcome
		if outcome == "" {
			outcome = "success"
			if status >= 400 {
				outcome = "error"
			}
		}
		level := slog.LevelInfo
		if outcome == "error" {
			level = slog.LevelWarn
		}
		route := r.Pattern
		if route == "" {
			route = "unmatched"
		}
		logger.LogAttrs(ctx, level, "http_request",
			slog.String("method", r.Method), slog.String("route", route),
			slog.Int("status", status), slog.String("outcome", outcome),
			slog.Int64("duration_ms", time.Since(started).Milliseconds()))
	})
}
