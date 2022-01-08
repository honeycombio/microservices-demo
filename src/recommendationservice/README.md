# recommendation service

The **recommendation** service is used to find a set of products to recommend, based on an existing set of products.

## OpenTelemetry instrumentation

### Initialization
The OpenTelemetry SDK is initialized in `__main__`
```python
    # create Resource attributes used by the OpenTelemetry SDK
    resource = Resource(attributes={
        "service.name": os.environ.get("SERVICE_NAME"),
        "service.version": "0.1", "ip": os.environ.get('POD_IP')
    })

    # create the OTLP exporter to send data an insecure OpenTelemetry Collector
    otlp_exporter = OTLPSpanExporter(
        endpoint=os.environ.get('OTEL_EXPORTER_OTLP_ENDPOINT'),
        insecure=True
    )

    # create a Trace Provider
    trace_provider = TracerProvider(resource=resource)
    trace_provider.add_span_processor(
        BatchSpanProcessor(otlp_exporter)
    )

    # set the Trace Provider to be used by the OpenTelemetry SDK
    trace.set_tracer_provider(trace_provider)
```

### gRPC instrumentation
This service sends and receives gRPC requests, which are instrumented in `__main__` as part of the gRPC client and server creation.
```python
    client_instrumentor = GrpcInstrumentorClient().instrument()
    server_instrumentor = GrpcInstrumentorServer().instrument()
```
