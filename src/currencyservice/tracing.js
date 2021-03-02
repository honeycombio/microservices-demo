"use strict";

const grpc = require('grpc');
const { NodeTracerProvider } = require("@opentelemetry/node");
const { SimpleSpanProcessor } = require("@opentelemetry/tracing");
const { CollectorTraceExporter } = require("@opentelemetry/exporter-collector-grpc");
const { registerInstrumentations } = require('@opentelemetry/instrumentation');
const provider = new NodeTracerProvider({
    plugins: {
      grpc: {
        enabled: true,
        // You may use a package name or absolute path to the file.
        path: '@opentelemetry/plugin-grpc',
        // gRPC plugin options
      }
    }
  });

const metadata = new grpc.Metadata();
metadata.set("x-honeycomb-team", process.env.HONEYCOMB_API_KEY);
metadata.set("x-honeycomb-dataset", process.env.HONEYCOMB_DATASET);

const collectorOptions = {
	serviceName: process.env.SERVICE_NAME,
	url: process.env.OTEL_EXPORTER_OTLP_ENDPOINT,
	credentials: grpc.credentials.createSsl(),
	metadata
  };

provider.addSpanProcessor(
  new SimpleSpanProcessor(
    new CollectorTraceExporter(collectorOptions)
  )
);

provider.register();
registerInstrumentations({
    tracerProvider: provider,
  });
console.log("tracing initialized");