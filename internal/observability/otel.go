package observability

import (
	"context"
	"fmt"
	"os"
	"sync"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.40.0"
)

var (
	tracerProvider *sdktrace.TracerProvider
	tracerOnce     sync.Once
)

func InitTracer(service string) (func(context.Context) error, error) {
	var initErr error
	tracerOnce.Do(func() {
		exporter, err := stdouttrace.New(
			stdouttrace.WithWriter(os.Stderr),
			stdouttrace.WithoutTimestamps(),
		)
		if err != nil {
			initErr = fmt.Errorf("init stdout exporter: %w", err)
			return
		}

		res, err := resource.Merge(
			resource.Default(),
			resource.NewWithAttributes(
				semconv.SchemaURL,
				semconv.ServiceName(service),
			),
		)
		if err != nil {
			initErr = fmt.Errorf("init resource: %w", err)
			return
		}

		tracerProvider = sdktrace.NewTracerProvider(
			sdktrace.WithBatcher(exporter),
			sdktrace.WithResource(res),
			sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(0.1))),
		)
		otel.SetTracerProvider(tracerProvider)
		otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
			propagation.TraceContext{},
			propagation.Baggage{},
		))
	})
	if initErr != nil {
		return nil, initErr
	}
	return func(ctx context.Context) error {
		if tracerProvider == nil {
			return nil
		}
		return tracerProvider.Shutdown(ctx)
	}, nil
}
