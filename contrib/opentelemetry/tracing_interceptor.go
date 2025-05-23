// Package opentelemetry provides OpenTelemetry utilities.
package opentelemetry

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/baggage"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"

	"go.temporal.io/sdk/interceptor"
	"go.temporal.io/sdk/log"
)

// DefaultTextMapPropagator is the default OpenTelemetry TextMapPropagator used
// by this implementation if not otherwise set in TracerOptions.
var DefaultTextMapPropagator = propagation.NewCompositeTextMapPropagator(
	propagation.TraceContext{},
	propagation.Baggage{},
)

// TracerOptions are options provided to NewTracingInterceptor or NewTracer.
type TracerOptions struct {
	// Tracer is the tracer to use. If not set, one is obtained from the global
	// tracer provider using the name "temporal-sdk-go".
	Tracer trace.Tracer

	// DisableSignalTracing can be set to disable signal tracing.
	DisableSignalTracing bool

	// DisableQueryTracing can be set to disable query tracing.
	DisableQueryTracing bool

	// DisableUpdateTracing can be set to disable update tracing.
	DisableUpdateTracing bool

	// DisableBaggage can be set to disable baggage propagation.
	DisableBaggage bool

	// AllowInvalidParentSpans will swallow errors interpreting parent
	// spans from headers. Useful when migrating from one tracing library
	// to another, while workflows/activities may be in progress.
	AllowInvalidParentSpans bool

	// TextMapPropagator is the propagator to use for serializing spans. If not
	// set, this uses DefaultTextMapPropagator, not the OpenTelemetry global one.
	// To use the OpenTelemetry global one, set this value to the result of the
	// global call.
	TextMapPropagator propagation.TextMapPropagator

	// SpanContextKey is the context key used for internal span tracking (not to
	// be confused with the context key OpenTelemetry uses internally). If not
	// set, this defaults to an internal key (recommended).
	SpanContextKey interface{}

	// HeaderKey is the Temporal header field key used to serialize spans. If
	// empty, this defaults to the one used by all SDKs (recommended).
	HeaderKey string

	// SpanStarter is a callback to create spans. If not set, this creates normal
	// OpenTelemetry spans calling Tracer.Start.
	SpanStarter func(ctx context.Context, t trace.Tracer, spanName string, opts ...trace.SpanStartOption) trace.Span
}

type spanContextKey struct{}

const defaultHeaderKey = "_tracer-data"

type tracer struct {
	interceptor.BaseTracer
	options *TracerOptions
}

// NewTracer creates a tracer with the given options. Most callers should use
// NewTracingInterceptor instead.
func NewTracer(options TracerOptions) (interceptor.Tracer, error) {
	if options.Tracer == nil {
		options.Tracer = otel.GetTracerProvider().Tracer("temporal-sdk-go")
	}
	if options.TextMapPropagator == nil {
		options.TextMapPropagator = DefaultTextMapPropagator
	}
	if options.SpanContextKey == nil {
		options.SpanContextKey = spanContextKey{}
	}
	if options.HeaderKey == "" {
		options.HeaderKey = defaultHeaderKey
	}
	if options.SpanStarter == nil {
		options.SpanStarter = func(
			ctx context.Context,
			t trace.Tracer,
			spanName string,
			opts ...trace.SpanStartOption,
		) trace.Span {
			_, span := t.Start(ctx, spanName, opts...)
			return span
		}
	}
	return &tracer{options: &options}, nil
}

// NewTracingInterceptor creates an interceptor for setting on client options
// that implements OpenTelemetry tracing for workflows.
func NewTracingInterceptor(options TracerOptions) (interceptor.Interceptor, error) {
	t, err := NewTracer(options)
	if err != nil {
		return nil, err
	}
	return interceptor.NewTracingInterceptor(t), nil
}

func (t *tracer) Options() interceptor.TracerOptions {
	return interceptor.TracerOptions{
		SpanContextKey:          t.options.SpanContextKey,
		HeaderKey:               t.options.HeaderKey,
		DisableSignalTracing:    t.options.DisableSignalTracing,
		DisableQueryTracing:     t.options.DisableQueryTracing,
		DisableUpdateTracing:    t.options.DisableUpdateTracing,
		AllowInvalidParentSpans: t.options.AllowInvalidParentSpans,
	}
}

func (t *tracer) UnmarshalSpan(m map[string]string) (interceptor.TracerSpanRef, error) {
	if _, ok := m["traceparent"]; !ok {
		// If there is no span, return nothing, but don't error out. This is
		// a legitimate place where a span does not exist in the headers
		return nil, nil
	}
	ctx := t.options.TextMapPropagator.Extract(context.Background(), textMapCarrier(m))
	spanCtx := trace.SpanContextFromContext(ctx)
	if !spanCtx.IsValid() {
		return nil, fmt.Errorf("failed extracting OpenTelemetry span from map")
	}
	spanRef := &tracerSpanRef{SpanContext: spanCtx}
	if !t.options.DisableBaggage {
		spanRef.Baggage = baggage.FromContext(ctx)
	}
	return spanRef, nil
}

func (t *tracer) MarshalSpan(span interceptor.TracerSpan) (map[string]string, error) {
	data := textMapCarrier{}
	tSpan := span.(*tracerSpan)
	ctx := context.Background()
	if !t.options.DisableBaggage {
		ctx = baggage.ContextWithBaggage(ctx, tSpan.Baggage)
	}
	t.options.TextMapPropagator.Inject(trace.ContextWithSpan(ctx, tSpan.Span), data)
	return data, nil
}

func (t *tracer) SpanFromContext(ctx context.Context) interceptor.TracerSpan {
	span := trace.SpanFromContext(ctx)
	if !span.SpanContext().IsValid() {
		return nil
	}
	tSpan := &tracerSpan{Span: span}
	if !t.options.DisableBaggage {
		tSpan.Baggage = baggage.FromContext(ctx)
	}
	return tSpan
}

func (t *tracer) ContextWithSpan(ctx context.Context, span interceptor.TracerSpan) context.Context {
	if !t.options.DisableBaggage {
		ctx = baggage.ContextWithBaggage(ctx, span.(*tracerSpan).Baggage)
	}
	return trace.ContextWithSpan(ctx, span.(*tracerSpan).Span)
}

func (t *tracer) StartSpan(opts *interceptor.TracerStartSpanOptions) (interceptor.TracerSpan, error) {
	// Create context with parent
	var parent trace.SpanContext
	var bag baggage.Baggage
	switch optParent := opts.Parent.(type) {
	case nil:
	case *tracerSpan:
		parent = optParent.SpanContext()
		bag = optParent.Baggage
	case *tracerSpanRef:
		parent = optParent.SpanContext
		bag = optParent.Baggage
	default:
		return nil, fmt.Errorf("unrecognized parent type %T", optParent)
	}
	ctx := context.Background()
	if parent.IsValid() {
		ctx = trace.ContextWithSpanContext(ctx, parent)
		if !t.options.DisableBaggage {
			ctx = baggage.ContextWithBaggage(ctx, bag)
		}
	}

	// Create span
	span := t.options.SpanStarter(ctx, t.options.Tracer, opts.Operation+":"+opts.Name, trace.WithTimestamp(opts.Time))

	// Set tags
	if len(opts.Tags) > 0 {
		attrs := make([]attribute.KeyValue, 0, len(opts.Tags))
		for k, v := range opts.Tags {
			attrs = append(attrs, attribute.String(k, v))
		}
		span.SetAttributes(attrs...)
	}

	tSpan := &tracerSpan{Span: span}
	if !t.options.DisableBaggage {
		tSpan.Baggage = bag
	}

	return tSpan, nil
}

func (t *tracer) GetLogger(logger log.Logger, ref interceptor.TracerSpanRef) log.Logger {
	span, ok := ref.(*tracerSpan)
	if !ok {
		return logger
	}

	logger = log.With(logger,
		"TraceID", span.SpanContext().TraceID(),
		"SpanID", span.SpanContext().SpanID(),
	)

	return logger
}

type tracerSpanRef struct {
	trace.SpanContext
	baggage.Baggage
}

type tracerSpan struct {
	trace.Span
	baggage.Baggage
}

func (t *tracerSpan) Finish(opts *interceptor.TracerFinishSpanOptions) {
	if opts.Error != nil {
		t.SetStatus(codes.Error, opts.Error.Error())
	}
	t.End()
}

type textMapCarrier map[string]string

func (t textMapCarrier) Get(key string) string        { return t[key] }
func (t textMapCarrier) Set(key string, value string) { t[key] = value }
func (t textMapCarrier) Keys() []string {
	ret := make([]string, 0, len(t))
	for k := range t {
		ret = append(ret, k)
	}
	return ret
}
