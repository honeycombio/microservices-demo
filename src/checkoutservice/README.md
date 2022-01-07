# checkout service

The **checkout** service provides cart management, and order placement functionality.

## OpenTelemetry instrumentation

The OpenTelemetry SDK is initialized in `main` using the `initOtelTracing` function.
This function contains the boilerplate code required to initialize a `TraceProvider`.
This service receives gRPC requests, which are instrumented in the `main` function as part of the gRPC server creation.
This service will issue several outgoing gRPC calls, which are all instrumented within their respective handlers.

This is the code that is used to instrument the gRPC server:
```go
	srv := grpc.NewServer(
		grpc.UnaryInterceptor(otelgrpc.UnaryServerInterceptor(otelgrpc.WithTracerProvider(otel.GetTracerProvider()))),
		grpc.StreamInterceptor(otelgrpc.StreamServerInterceptor(otelgrpc.WithTracerProvider(otel.GetTracerProvider()))),
	)
```

An example of how to instrument the outgoing gRPC calls:
```go
	conn, err := grpc.DialContext(ctx, cs.shippingSvcAddr,
		grpc.WithInsecure(),
		grpc.WithUnaryInterceptor(otelgrpc.UnaryClientInterceptor(otelgrpc.WithTracerProvider(otel.GetTracerProvider()))),
		grpc.WithStreamInterceptor(otelgrpc.StreamClientInterceptor(otelgrpc.WithTracerProvider(otel.GetTracerProvider()))))
```

### Baggage 

This service makes use of information from Baggage, which originate from an upstream service (frontend).
The `PlaceOrder` function makes use of Baggage to both read, and set Members on it.

## Demo Story code

In order to produce an effective demo story, this service includes additional functionality.
This service will grow an internal cache with each request.
Eventually, the cache size will grow large enough to cause an out of memory (OOM) error and crash the service.
The cache size is exposed via an internal call, so the frontend can properly assign a problematic userid when a cache size threshold is reached.
When an order is placed, an additional delay through `getDiscounts` may be introduced.
In this function, if a random chance exists to call the `loadDiscountFromDatabase` function, which will introduce a synthetic delay based on cache size.
The synthetic delay is manifested as a series of spans making mock database calls.
User "20109", whom should only show up when cache size is high, will have a higher likelihood to exhibit the delay.
