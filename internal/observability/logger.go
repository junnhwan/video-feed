package observability

import (
	"context"
	"os"
	"sync"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	globalLogger *zap.Logger
	loggerOnce   sync.Once
)

type loggerKey struct{}

func InitLogger(service string) *zap.Logger {
	loggerOnce.Do(func() {
		encoderConfig := zap.NewProductionEncoderConfig()
		encoderConfig.TimeKey = "ts"
		encoderConfig.MessageKey = "msg"
		encoderConfig.LevelKey = "level"
		encoderConfig.CallerKey = "caller"
		encoderConfig.StacktraceKey = "stack"
		encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
		encoderConfig.EncodeDuration = zapcore.MillisDurationEncoder

		core := zapcore.NewCore(
			zapcore.NewJSONEncoder(encoderConfig),
			zapcore.Lock(os.Stdout),
			zapcore.InfoLevel,
		)
		globalLogger = zap.New(core,
			zap.AddCaller(),
			zap.AddCallerSkip(0),
			zap.AddStacktrace(zapcore.ErrorLevel),
			zap.Fields(zap.String("service", service)),
		)
		zap.ReplaceGlobals(globalLogger)
		_ = zap.RedirectStdLog(globalLogger)
	})
	return globalLogger
}

func L() *zap.Logger {
	if globalLogger == nil {
		return zap.NewNop()
	}
	return globalLogger
}

func WithContext(ctx context.Context, fields ...zap.Field) *zap.Logger {
	logger := L()
	if ctx != nil {
		if v := ctx.Value(loggerKey{}); v != nil {
			if l, ok := v.(*zap.Logger); ok {
				logger = l
			}
		}
		if traceID := TraceIDFromContext(ctx); traceID != "" {
			logger = logger.With(zap.String("trace_id", traceID))
		}
	}
	if len(fields) > 0 {
		logger = logger.With(fields...)
	}
	return logger
}

func ContextWithLogger(ctx context.Context, logger *zap.Logger) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, loggerKey{}, logger)
}

func Sync() {
	if globalLogger != nil {
		_ = globalLogger.Sync()
	}
}
