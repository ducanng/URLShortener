// Package logger provides a structured JSON logger built on uber-go/zap and
// designed for dependency injection. A single *Logger is constructed in main
// and embedded into the structs that need to log (storage, client, server),
// so business code never reaches for a global.
//
// Three logging styles are supported:
//
//   - Typed (zero-alloc, hot path):
//     l.Info("starting", zap.String("addr", ":8080"))
//
//   - Printf-style (cold path, ergonomic):
//     l.Infof("starting on %s", addr)
//
//   - Structured key-value (logfmt-ish):
//     l.Infow("grpc call", "method", m, "duration", d)
//
// Trace-ID propagation lives in context.go. Use l.WithContext(ctx) to derive
// a child logger that automatically tags every record with the request's
// trace_id field.
package logger

import (
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Logger wraps a zap typed logger and a cached sugared logger. The embed
// exposes the typed API (Info/Warn/Error/Fatal with zap.Field) directly,
// while the helpers below cover printf and key-value styles without forcing
// callers to remember `.Sugar()`.
type Logger struct {
	*zap.Logger
	sugar *zap.SugaredLogger
}

// Config controls construction. Keep it small; add fields as real needs
// appear (output paths, sampling, encoder choice).
type Config struct {
	// Level is the minimum level emitted. Zero value = InfoLevel.
	Level zapcore.Level
}

// New builds a JSON-on-stdout logger. Returns an error so the caller (main)
// owns the decision of how to react — never call os.Exit/log.Fatal here.
func New(cfg Config) (*Logger, error) {
	enc := zap.NewProductionEncoderConfig()
	enc.TimeKey = "ts"
	enc.EncodeTime = zapcore.ISO8601TimeEncoder
	enc.EncodeLevel = zapcore.LowercaseLevelEncoder

	core := zapcore.NewCore(
		zapcore.NewJSONEncoder(enc),
		zapcore.AddSync(os.Stdout),
		cfg.Level,
	)

	z := zap.New(core, zap.AddCaller(), zap.AddStacktrace(zap.ErrorLevel))
	// AddCallerSkip(1) on sugar so printf-style helpers report the user's
	// call site, not logger/logger.go.
	return &Logger{Logger: z, sugar: z.WithOptions(zap.AddCallerSkip(1)).Sugar()}, nil
}

// Sync flushes buffered entries. Best-effort: stdout often returns
// EINVAL/ENOTTY, which is fine to ignore.
func (l *Logger) Sync() { _ = l.Logger.Sync() }

// Printf-style helpers. Use when you have format directives (%v, %s, %d).
func (l *Logger) Debugf(format string, args ...any) { l.sugar.Debugf(format, args...) }
func (l *Logger) Infof(format string, args ...any)  { l.sugar.Infof(format, args...) }
func (l *Logger) Warnf(format string, args ...any)  { l.sugar.Warnf(format, args...) }
func (l *Logger) Errorf(format string, args ...any) { l.sugar.Errorf(format, args...) }

// Fatalf logs at Fatal level then calls os.Exit(1). Use only from main.
func (l *Logger) Fatalf(format string, args ...any) { l.sugar.Fatalf(format, args...) }

// Structured key-value helpers. Pairs of (key, value, key, value, ...).
// Cheaper than Sprintf when fields are simple values.
func (l *Logger) Debugw(msg string, kv ...any) { l.sugar.Debugw(msg, kv...) }
func (l *Logger) Infow(msg string, kv ...any)  { l.sugar.Infow(msg, kv...) }
func (l *Logger) Warnw(msg string, kv ...any)  { l.sugar.Warnw(msg, kv...) }
func (l *Logger) Errorw(msg string, kv ...any) { l.sugar.Errorw(msg, kv...) }
