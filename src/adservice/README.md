# ad service

The **ad** service provides advertisement based on context keys. If no context keys are provided then it returns random ads.

## OpenTelemetry instrumentation

### Initialization
In Java, the OpenTelemetry SDK is initialized using a Java agent and environment variables, or Java system command line properties.
In this example we use the [Honeycomb OpenTelemetry Distro for Java](https://github.com/honeycombio/honeycomb-opentelemetry-java).
This is a wrapper to the standard OpenTelemetry Java agent, providing shortcuts to configure for Honeycomb output, and attach a Span processor which will export Baggage members as Span attributes.
In this example, we are using an OpenTelmetry Collector in our telemetry pipeline, the following environment variables are used to point ot that configuration:
- HONEYCOMB_API_ENDPOINT=http://opentelemetry-collector:4317
- SERVICE_NAME=adservice

Other environment variables may be required with other configurations, please refer to the Honeycomb [documentation](https://docs.honeycomb.io/getting-data-in/java/opentelemetry-distro/) for usage of this agent.

### Instrumenting with the @WithSpan annotation
The OpenTelemetry Java SDK provides annotations to facilitate instrumentation.
Adding the `@WithSpan` annotation to any function will wrap that function's execution within a span.
You can specify the emitted span name using the `@WithSpan("my-name")` syntax.
If not specified, the class and function name will be used as the span name.
The `getAds` and `getRandomAds` functions have this annotation.
```java
    // Wrap function in an OpenTelemetry span
    @WithSpan  //results in a span name of AdServiceImpl.getAds
    public void getAds(AdRequest req, StreamObserver<AdResponse> responseObserver) {
        // ...
    }

    // Wrap function in an OpenTelemetry span
    @WithSpan("random-ads")
    private List<Ad> getRandomAds() {
        // ...
    }
```

### Span attributes and events
With the OpenTelemetry Java SDK, you can leverage existing spans that are auto-instrumented, or manually created using `Span.current()`.
This technique is used in the `getAds` function so attributes and events can be added to the span.
```java
    // Get the current OpenTelemetry span
    Span span = Span.current();
```

Attributes can be added to the span using `Span.setAttribute()`
```java
    // Add a span attribute
    span.setAttribute("method", "getAds");
```

Span events can be created using `Span.addEvent()`
```java
    // Add a span event
    span.addEvent(
            "Constructing Ads using context",
            io.opentelemetry.api.common.Attributes.of(
                    AttributeKey.stringKey("Context Keys"), req.getContextKeysList().toString(),
                    AttributeKey.longKey("Context Keys length"), keyCount));
```
