package telemetry

import (
	"context"
	"os"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
)

func Start(ctx context.Context) (func(context.Context) error, error) {
	otel.SetTextMapPropagator(propagation.TraceContext{})
	if os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT") == "" {
		return func(context.Context) error { return nil }, nil
	}
	exp, e := otlptracehttp.New(ctx)
	if e != nil {
		return nil, e
	}
	tp := trace.NewTracerProvider(trace.WithBatcher(exp), trace.WithSampler(trace.ParentBased(trace.TraceIDRatioBased(.1))), trace.WithResource(resource.NewSchemaless(attribute.String("service.name", "afterglow"))))
	otel.SetTracerProvider(tp)
	return tp.Shutdown, nil
}
