# Shipping Service

The Shipping service provides price quote, tracking IDs, and the impression of order fulfillment & shipping processes.

## OpenTelemetry instrumentation

The OpenTelemetry SDK is initialized in `main` using the `initOtelTracing` function.
This function contains the boilerplate code required to initialize a `TraceProvider`.
This service receives gRPC requests, which are instrumented in the `main` function as part of the gRPC server creation.

This is the code that is used to instrument the gRPC server:
```go
	srv = grpc.NewServer(
		grpc.UnaryInterceptor(otelgrpc.UnaryServerInterceptor(otelgrpc.WithTracerProvider(otel.GetTracerProvider()))),
		grpc.StreamInterceptor(otelgrpc.StreamServerInterceptor(otelgrpc.WithTracerProvider(otel.GetTracerProvider()))),
	)
```
