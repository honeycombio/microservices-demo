"use strict";

const process = require('process');
const { NodeSDK } = require('@opentelemetry/sdk-node');
const { OTLPTraceExporter } = require('@opentelemetry/exporter-trace-otlp-grpc');
const { getNodeAutoInstrumentations } = require('@opentelemetry/auto-instrumentations-node');
const { Resource } = require('@opentelemetry/resources');
const { SEMRESATTRS_SERVICE_NAME } = require('@opentelemetry/semantic-conventions');


// Create an OpenTelemetry Collector exporter for traces
const traceExporter = new OTLPTraceExporter({
  url: process.env.OTEL_EXPORTER_OTLP_ENDPOINT
});

// create the OpenTelemetry NodeSDK trace provider
const sdk = new NodeSDK({
  resource: new Resource({
    [SEMRESATTRS_SERVICE_NAME]: process.env.SERVICE_NAME || 'payment',
    [ 'ip' ]: process.env.POD_IP,
  }),
  traceExporter,
  instrumentations: [getNodeAutoInstrumentations(
    {
      '@opentelemetry/instrumentation-fs': {
        enabled: false, // disable fs events
      }
    }
  )]
});

// Start the OpenTelemetry tracing SDK
try {
  sdk.start();
  console.log('Tracing initialized');
} catch (error) {
  console.log('Error initializing tracing', error);
}

// On shutdown, ensure we flush all telemetry first
process.on('SIGTERM', () => {
  sdk.shutdown()
      .then(() => console.log('Tracing terminated'))
      .catch((error) => console.log('Error terminating tracing', error))
      .finally(() => process.exit(0));
});
