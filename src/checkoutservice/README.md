# checkout service

The **checkout** service provides cart management, and order placement functionality.

## OpenTelemetry instrumentation

### Initialization
The OpenTelemetry SDK is initialized in `main` using the `initOtelTracing` function
```
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
		semconv.ServiceNameKey.String("checkout"),
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
```
	// Initialize OpenTelemetry Tracing
	ctx := context.Background()
	tp := initOtelTracing(ctx, log)
	defer func() { _ = tp.Shutdown(ctx) }()
```

### gRPC instrumentation
This service receives gRPC requests, which are instrumented in the `main` function as part of the gRPC server creation.
```
	// create gRPC server with OpenTelemetry instrumentation on all incoming requests
    srv := grpc.NewServer(
        grpc.UnaryInterceptor(otelgrpc.UnaryServerInterceptor(otelgrpc.WithTracerProvider(otel.GetTracerProvider()))),
        grpc.StreamInterceptor(otelgrpc.StreamServerInterceptor(otelgrpc.WithTracerProvider(otel.GetTracerProvider()))),
    )
```

This service will issue several outgoing gRPC calls, which are all instrumented within their respective handlers.
```go
	// add OpenTelemetry instrumentation to outgoing gRPC requests
	conn, err := grpc.DialContext(ctx, cs.shippingSvcAddr,
		grpc.WithInsecure(),
		grpc.WithUnaryInterceptor(otelgrpc.UnaryClientInterceptor(otelgrpc.WithTracerProvider(otel.GetTracerProvider()))),
		grpc.WithStreamInterceptor(otelgrpc.StreamClientInterceptor(otelgrpc.WithTracerProvider(otel.GetTracerProvider()))))
```

### Baggage 
This service makes use of information from Baggage, which originate from an upstream service (frontend).
The `PlaceOrder` function makes use of Baggage to both read, and set Members on it.
Reading userid and requestid from Baggage
```go
	// get userID and requestsID from Tracing Baggage
	bags := baggage.FromContext(ctx)
	userID := bags.Member("userid").Value()
	requestID := bags.Member("requestID").Value()
```

Setting orderid into Baggage
```go
	// Add orderid to Tracing Baggage
	orderIDMember, _ := baggage.NewMember("orderid", orderID.String())
	bags, _ = bags.SetMember(orderIDMember)
	ctx = baggage.ContextWithBaggage(ctx, bags)
```

## Demo Story code

In order to produce an effective demo story, this service includes additional functionality.
This service will grow an internal cache with each request.
Eventually, the cache size will grow large enough to cause an out of memory (OOM) error and crash the service.
The cache size is exposed via an internal call, so the frontend can properly assign a problematic userid when a cache size threshold is reached.
When an order is placed, an additional delay through `getDiscounts` may be introduced.
In this function, if a random chance exists to call the `loadDiscountFromDatabase` function, which will introduce a synthetic delay based on cache size.
The synthetic delay is manifested as a series of spans making mock database calls.
User "20109", whom should only show up when cache size is high, will have a higher likelihood to exhibit the delay.
