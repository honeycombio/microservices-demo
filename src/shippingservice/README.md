# shipping service

The **shipping** service provides price quote, tracking IDs, and the impression of order fulfillment & shipping processes.

## OpenTelemetry instrumentation

### Initialization
The OpenTelemetry SDK is initialized in `main` using the `initOtelTracing` function
```go
func initOtelTracing(ctx context.Context, log logrus.FieldLogger) *sdktrace.TracerProvider {
	endpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	if endpoint == "" {
		endpoint = "opentelemetry-collector:4317"
	}

	// Set GRPC options to establish an insecure connection to an OpenTelemetry Collector
	// To establish a TLS connection to a secured endpoint use:
	//   otlptracegrpc.WithTLSCredentials(credentials.NewClientTLSFromCert(nil, ""))
	opts := []otlptracegrpc.Option{
		otlptracegrpc.WithEndpoint(endpoint),
		otlptracegrpc.WithInsecure(),
	}

	// Create the exporter
	exporter, err := otlptrace.New(ctx, otlptracegrpc.NewClient(opts...))
	if err != nil {
		log.Fatal(err)
	}

	// Specify the TextMapPropagator to ensure spans propagate across service boundaries
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.Baggage{}, propagation.TraceContext{}))

	// Set standard attributes per semantic conventions
	res := resource.NewWithAttributes(
		semconv.SchemaURL,
		semconv.ServiceNameKey.String("shipping"),
	)

	// Create and set the TraceProvider
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(tp)

	return tp
}
```

You should call `TraceProvider.shutdown()` when your service is shutdown to ensure all spans are exported.
This service makes that call as part of a deferred function in `main`
```go
	// Initialize OpenTelemetry Tracing
	ctx := context.Background()
	tp := initOtelTracing(ctx, log)
	defer func() { _ = tp.Shutdown(ctx) }()
```

### gRPC instrumentation
This service receives gRPC requests, which are instrumented in the `main` function as part of the gRPC server creation.
```go
	// create gRPC server with OpenTelemetry instrumentation on all incoming requests
    srv := grpc.NewServer(
        grpc.UnaryInterceptor(otelgrpc.UnaryServerInterceptor(otelgrpc.WithTracerProvider(otel.GetTracerProvider()))),
        grpc.StreamInterceptor(otelgrpc.StreamServerInterceptor(otelgrpc.WithTracerProvider(otel.GetTracerProvider()))),
    )    
```

