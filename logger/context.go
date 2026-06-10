package logger

import (
	"context"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// ctxKey is unexported so callers cannot collide on the same key from another
// package. The single-iota constant is enough — we only carry a trace_id today.
type ctxKey int

const traceIDKey ctxKey = iota

// WithTraceID returns a child context carrying traceID. Use this in middleware
// or interceptors at the entry of a request.
func WithTraceID(ctx context.Context, traceID string) context.Context {
	return context.WithValue(ctx, traceIDKey, traceID)
}

// TraceIDFromContext returns the trace_id stored in ctx, if any. The boolean
// is true only when a non-empty trace_id is present, so callers can fall
// back without inspecting the string value.
func TraceIDFromContext(ctx context.Context) (string, bool) {
	s, ok := ctx.Value(traceIDKey).(string)
	return s, ok && s != ""
}

// NewTraceID generates a fresh UUIDv4 string. Used by request entry points
// when the upstream caller did not supply an X-Trace-Id / x-trace-id header.
func NewTraceID() string { return uuid.NewString() }

// WithContext returns a child Logger augmented with the trace_id from ctx.
// If ctx has no trace_id, the receiver is returned as-is — no allocation.
//
// Typical use inside a method that already accepts ctx:
//
//	func (s *SQLStore) Save(ctx context.Context, e Entry) error {
//	    log := s.WithContext(ctx)
//	    log.Infof("saving id=%d", e.Id)
//	    ...
//	}
func (l *Logger) WithContext(ctx context.Context) *Logger {
	id, ok := TraceIDFromContext(ctx)
	if !ok {
		return l
	}
	z := l.Logger.With(zap.String("trace_id", id))
	// Mirror the caller-skip applied in New() so printf helpers continue to
	// report the user's call site after the fork.
	return &Logger{Logger: z, sugar: z.WithOptions(zap.AddCallerSkip(1)).Sugar()}
}
