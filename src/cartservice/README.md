# cart service

The **cart** service provides shopping cart functionality, such as getting a cart contents, adding items to a cart, or emptying the cart.

## OpenTelemetry instrumentation

### Initialization
The OpenTelemetry SDK is initialized the the `ConfigureServices` function in `Startup.cs`.
As part of this initialization, AspNetCore and HttpClient instrumentation is added for all incoming requests.
```cs
    // Initialize the OpenTelemetry tracing API, with resource attributes
    // setup instrumentation for AspNetCore and HttpClient
    // use the OTLP exporter
    services.AddOpenTelemetryTracing((builder) => builder
        .SetResourceBuilder(ResourceBuilder.CreateDefault().AddService(servicename).AddAttributes(attributes))
        .AddAspNetCoreInstrumentation()
        .AddHttpClientInstrumentation()
        .AddOtlpExporter(otlpOptions =>
        {
            otlpOptions.Endpoint = new Uri(otlpendpoint);
            var headers = new Grpc.Core.Metadata();
            otlpOptions.Headers = headers;
        }));
```

