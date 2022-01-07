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

The OpenTelemetry SDK is initialized in `main` using the `initOtelTracing` function.
This function contains the boilerplate code required to initialize a `TraceProvider`.
All outgoing gRPC calls have instrumentation hooks, enabled by the `mustConnGRPC` function.
This function is used to wrap the gRPC connections for all downstream services that Frontend will call.

### Baggage

This service will add some telemetry data to OpenTelemetry `Baggage`, which is propagated to downstream services.
The `placeOrderHandler` in the `handlers.go` file will add the userid and requestid to baggage.
The userid and requestid will be available to all downstream spans.
Baggage is propagated to downstream services, but by default it is not exported to your telemetry backend.
A Span Processor that explicitly exports Baggage is required to export this data to a telemetry backend like Honeycomb.

## Demo Story code

In order to produce an effective demo story, this service includes additional functionality.
The `ensureSessionID` function in `middleware.go` assigns user ids in a random fashion, which may be affected under other random condition, when the cache size from the checkout service exceeds a threshold.
The application will enter a degraded state of performance when cache size climbs.
The checkout service has code to continuously grow a cache, until memory is exhausted and the service crashes with an out of memory (OOM) error.