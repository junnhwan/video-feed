package observability

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

const TracerName = "video-feed"

func Tracer() trace.Tracer {
	return otel.Tracer(TracerName)
}

func TraceIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	span := trace.SpanContextFromContext(ctx)
	if !span.IsValid() {
		return ""
	}
	return span.TraceID().String()
}

func SpanIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	span := trace.SpanContextFromContext(ctx)
	if !span.IsValid() {
		return ""
	}
	return span.SpanID().String()
}
