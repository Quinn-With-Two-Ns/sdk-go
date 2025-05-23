// Package opentracing provides OpenTracing utilities.
package opentracing

import (
	"context"
	"errors"
	"fmt"

	"github.com/opentracing/opentracing-go"

	"go.temporal.io/sdk/interceptor"
)

// TracerOptions are options provided to NewInterceptor or NewTracer.
type TracerOptions struct {
	// Tracer is the tracer to use. If not set, the global one is used.
	Tracer opentracing.Tracer

	// DisableSignalTracing can be set to disable signal tracing.
	DisableSignalTracing bool

	// DisableQueryTracing can be set to disable query tracing.
	DisableQueryTracing bool

	// SpanContextKey is the context key used for internal span tracking (not to
	// be confused with the context key OpenTracing uses internally). If not set,
	// this defaults to an internal key (recommended).
	SpanContextKey interface{}

	// HeaderKey is the Temporal header field key used to serialize spans. If
	// empty, this defaults to the one used by all SDKs (recommended).
	HeaderKey string

	// SpanStarter is a callback to create spans. If not set, this creates normal
	// OpenTracing spans calling Tracer.StartSpan.
	SpanStarter func(t opentracing.Tracer, operationName string, opts ...opentracing.StartSpanOption) opentracing.Span
}

type spanContextKey struct{}

const defaultHeaderKey = "_tracer-data"

type tracer struct {
	interceptor.BaseTracer
	options *TracerOptions
}

// NewTracer creates a tracer with the given options. Most callers should use
// NewInterceptor instead.
func NewTracer(options TracerOptions) (interceptor.Tracer, error) {
	if options.Tracer == nil {
		options.Tracer = opentracing.GlobalTracer()
	}
	if options.SpanContextKey == nil {
		options.SpanContextKey = spanContextKey{}
	}
	if options.HeaderKey == "" {
		options.HeaderKey = defaultHeaderKey
	}
	if options.SpanStarter == nil {
		options.SpanStarter = func(
			t opentracing.Tracer,
			operationName string,
			opts ...opentracing.StartSpanOption,
		) opentracing.Span {
			return t.StartSpan(operationName, opts...)
		}
	}
	return &tracer{options: &options}, nil
}

// NewTracingInterceptor creates an interceptor for setting on client options
// that implements OpenTracing tracing for workflows.
func NewInterceptor(options TracerOptions) (interceptor.Interceptor, error) {
	t, err := NewTracer(options)
	if err != nil {
		return nil, err
	}
	return interceptor.NewTracingInterceptor(t), nil
}

func (t *tracer) Options() interceptor.TracerOptions {
	return interceptor.TracerOptions{
		SpanContextKey:       t.options.SpanContextKey,
		HeaderKey:            t.options.HeaderKey,
		DisableSignalTracing: t.options.DisableSignalTracing,
		DisableQueryTracing:  t.options.DisableQueryTracing,
	}
}

func (t *tracer) UnmarshalSpan(m map[string]string) (interceptor.TracerSpanRef, error) {
	ctx, err := t.options.Tracer.Extract(opentracing.TextMap, opentracing.TextMapCarrier(m))
	if errors.Is(err, opentracing.ErrSpanContextNotFound) {
		// If there is no span, return nothing, but don't error out. This is
		// a legitimate place where a span does not exist in the headers
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &tracerSpanRef{SpanContext: ctx}, nil
}

func (t *tracer) MarshalSpan(span interceptor.TracerSpan) (map[string]string, error) {
	data := opentracing.TextMapCarrier{}
	if err := t.options.Tracer.Inject(span.(*tracerSpan).Context(), opentracing.TextMap, data); err != nil {
		return nil, err
	}
	return map[string]string(data), nil
}

func (t *tracer) SpanFromContext(ctx context.Context) interceptor.TracerSpan {
	span := opentracing.SpanFromContext(ctx)
	if span == nil {
		return nil
	}
	return &tracerSpan{Span: span}
}

func (t *tracer) ContextWithSpan(ctx context.Context, span interceptor.TracerSpan) context.Context {
	return opentracing.ContextWithSpan(ctx, span.(*tracerSpan).Span)
}

func (t *tracer) StartSpan(opts *interceptor.TracerStartSpanOptions) (interceptor.TracerSpan, error) {
	// Build start options
	startOpts := []opentracing.StartSpanOption{
		opentracing.StartTime(opts.Time),
	}

	// Link parent
	var parent opentracing.SpanContext
	switch optParent := opts.Parent.(type) {
	case nil:
	case *tracerSpan:
		parent = optParent.Context()
	case *tracerSpanRef:
		parent = optParent.SpanContext
	default:
		return nil, fmt.Errorf("unrecognized parent type %T", optParent)
	}
	if parent != nil {
		if opts.DependedOn {
			startOpts = append(startOpts, opentracing.ChildOf(parent))
		} else {
			startOpts = append(startOpts, opentracing.FollowsFrom(parent))
		}
	}

	// Set tags
	if len(opts.Tags) > 0 {
		tags := make(opentracing.Tags, len(opts.Tags))
		for k, v := range opts.Tags {
			tags[k] = v
		}
		startOpts = append(startOpts, tags)
	}

	// Start
	return &tracerSpan{Span: t.options.SpanStarter(t.options.Tracer, opts.Operation+":"+opts.Name, startOpts...)}, nil
}

type tracerSpanRef struct{ opentracing.SpanContext }

type tracerSpan struct{ opentracing.Span }

func (t *tracerSpan) Finish(opts *interceptor.TracerFinishSpanOptions) {
	if opts.Error != nil {
		// Standard tag that can be bridged to OpenTelemetry
		t.SetTag("error", "true")
	}
	t.Span.Finish()
}
