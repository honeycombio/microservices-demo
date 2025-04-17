# checkout service

The **checkout** service provides cart management, and order placement functionality.

## OpenTelemetry instrumentation

### Initialization
The OpenTelemetry SDK is initialized in `main` using the `initOtelLogging` function and `initOtelTracing` function
```go
func initOtelLogging(ctx context.Context) *sdklog.LoggerProvider {
	endpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	if endpoint == "" {
		endpoint = "opentelemetry-collector:4317"
	}
	logExporter, err := otlploggrpc.New(
		ctx,
		otlploggrpc.WithEndpoint(endpoint),
	)
	if err != nil {
		log.Fatal(err)
	}
	res := resource.NewWithAttributes(
		semconv.SchemaURL,
		semconv.ServiceNameKey.String("checkout"),
	)
	provider := sdklog.NewLoggerProvider(
		sdklog.WithProcessor(
			sdklog.NewBatchProcessor(logExporter),
		),
		sdklog.WithResource(res),
	)
	global.SetLoggerProvider(provider)
	return provider
}

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

The [logrus](https://github.com/sirupsen/logrus) package is added with OtelHook which generates Otel Logs.

```go
// OtelHook is a logrus hook that sends logs to OpenTelemetry
type OtelHook struct{}

func (h *OtelHook) Levels() []logrus.Level {
	return logrus.AllLevels
}

func (h *OtelHook) Fire(entry *logrus.Entry) error {
	logger := global.Logger("logrus-otel")
	attrs := []otelLog.KeyValue{}
	if entry.Data["trace_id"] != nil && entry.Data["span_id"] != nil {
		attrs = []otelLog.KeyValue{
			otelLog.String("trace.trace_id", entry.Data["trace_id"].(string)),
			otelLog.String("trace.parent_id", entry.Data["span_id"].(string)),
			otelLog.String("meta.annotation_type", "span_event"),
		}
	}
	if entry.Data["error"] != nil {
		attrs = append(attrs, otelLog.String("error.message", fmt.Sprintf("%+v", entry.Data["error"])))
	}
	var record otelLog.Record
	record.AddAttributes(attrs...)
	record.SetBody(otelLog.StringValue(entry.Message))
	if entry.Data["error"] != nil {
		record.SetSeverity(otelLog.SeverityError)
	} else {
		record.SetSeverity(otelLog.Severity(entry.Level))
	}
	record.SetTimestamp(entry.Time)
	logger.Emit(context.Background(), record)
	// fmt.Println("** OTELHOOK triggered for log level:", entry.Level)
	return nil
}

func init() {
	log = logrus.New()
	log.Level = logrus.DebugLevel
	log.Formatter = &logrus.JSONFormatter{
		FieldMap: logrus.FieldMap{
			logrus.FieldKeyTime:  "timestamp",
			logrus.FieldKeyLevel: "severity",
			logrus.FieldKeyMsg:   "message",
		},
		TimestampFormat: time.RFC3339Nano,
	}
	log.Out = os.Stdout
	log.AddHook(&OtelHook{}) // Otel Hook is added to logrus to send logs to OpenTelemetry
}
```

You should call `TraceProvider.shutdown()` when your service is shutdown to ensure all spans are exported.
This service makes that call as part of a deferred function in `main`
```
	// Initialize OpenTelemetry Log and Tracing
	ctx := context.Background()
	lp := initOtelLogging(ctx)
	defer func() { _ = lp.Shutdown(ctx) }()
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
	orderIDMember, _ := baggage.NewMember("app.order_id", orderID.String())
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
