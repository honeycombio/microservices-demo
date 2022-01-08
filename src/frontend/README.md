# frontend service

The **frontend** service is responsible for rendering the UI for the store's website.
It serves as the main entry point for the application.
The application uses Server Side Rendering (SSR) to generate HTML consumed by the browser.

The following routes are defined by the frontend:

| Path              | Method | Use                               |
|-------------------|--------|-----------------------------------|
| `/`               | GET    | Main index page                   |
| `/cart`           | GET    | View Cart                         |
| `/cart`           | POST   | Add to Cart                       |
| `/cart/checktout` | POST   | Place Order                       |
| `/cart/empty`     | POST   | Empty Cart                        |
| `/logout`         | GET    | Logout                            |
| `/product/{id}`   | GET    | View Product                      |
| `/setCurrency`    | POST   | Set Currency                      |
| `/static/`        | *      | Static resources                  |
| `/dist/`          | *      | Compiled Javascript resources     |
| `/robots.txt`     | *      | Search engine response (disallow) |
| `/_healthz`       | *      | Health check (ok)                 |

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

### HTTP instrumentation
This service recieves HTTP requests, controlled by the gorilla/mux Router.
These requests are instrumented in the main function as part of the router's definition.
```go
	// Add OpenTelemetry instrumentation to incoming HTTP requests controlled by the gorilla/mux Router.
	r.Use(middleware.Middleware("frontend"))
```

### gRPC instrumentation
This service will issue several outgoing gRPC calls, which have instrumentation hooks added in the `mustConnGRPC` function.
```go
	// add OpenTelemetry instrumentation to outgoing gRPC requests
    var err error
    *conn, err = grpc.DialContext(ctx, addr,
        grpc.WithTransportCredentials(insecure.NewCredentials()),
        grpc.WithUnaryInterceptor(otelgrpc.UnaryClientInterceptor(otelgrpc.WithTracerProvider(otel.GetTracerProvider()))),
        grpc.WithStreamInterceptor(otelgrpc.StreamClientInterceptor(otelgrpc.WithTracerProvider(otel.GetTracerProvider()))),
	)
```

### Baggage
This service will add some telemetry data to OpenTelemetry `Baggage`, which is propagated to downstream services.
The `placeOrderHandler` in the `handlers.go` file will add the userid and requestid to baggage.
The userid and requestid will be available to all downstream spans.
```go
	// add the UserID and requestId into OpenTelemetry Baggage to propagate across services
	userIdMember, _ := baggage.NewMember("userid", s)
	requestIdMember, _ := baggage.NewMember("requestID", reqID)
	bags := baggage.FromContext(ctx)
	bags, _ = bags.SetMember(userIdMember)
	bags, _ = bags.SetMember(requestIdMember)
	ctx = baggage.ContextWithBaggage(ctx, bags)
```

Baggage is propagated to downstream services, but by default it is not exported to your telemetry backend.
A Span Processor that explicitly exports Baggage is required to export this data to a telemetry backend like Honeycomb.

## Demo Story code

In order to produce an effective demo story, this service includes additional functionality.
The `ensureSessionID` function in `middleware.go` assigns user ids in a random fashion, which may be affected under other random condition, when the cache size from the checkout service exceeds a threshold.
The application will enter a degraded state of performance when cache size climbs.
The checkout service has code to continuously grow a cache, until memory is exhausted and the service crashes with an out of memory (OOM) error.