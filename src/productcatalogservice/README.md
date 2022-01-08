# productcatalog service

The **productcatalog** service provides details about all products in the application.

## Dynamic catalog reloading / artificial delay

This service has a "dynamic catalog reloading" feature that is purposefully
not well implemented. The goal of this feature is to allow you to modify the
`products.json` file and have the changes be picked up without having to
restart the service.

However, this feature is bugged: the catalog is actually reloaded on each
request, introducing a noticeable delay in the frontend. This delay will also
show up in profiling tools: the `parseCatalog` function will take more than 80%
of the CPU time.

You can trigger this feature (and the delay) by sending a `USR1` signal and
remove it (if needed) by sending a `USR2` signal:

```shell
# Trigger bug
kubectl exec \
    $(kubectl get pods -l app=productcatalogservice -o jsonpath='{.items[0].metadata.name}') \
    -c server -- kill -USR1 1
# Remove bug
kubectl exec \
    $(kubectl get pods -l app=productcatalogservice -o jsonpath='{.items[0].metadata.name}') \
    -c server -- kill -USR2 1
```

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
		semconv.ServiceNameKey.String("productcatalog"),
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
This service receives gRPC requests, which are instrumented in the `run` function as part of the gRPC server creation.
```go
	// create gRPC server with OpenTelemetry instrumentation on all incoming requests
	srv := grpc.NewServer(
		grpc.UnaryInterceptor(otelgrpc.UnaryServerInterceptor(otelgrpc.WithTracerProvider(otel.GetTracerProvider()))),
		grpc.StreamInterceptor(otelgrpc.StreamServerInterceptor(otelgrpc.WithTracerProvider(otel.GetTracerProvider()))),
	)
```
