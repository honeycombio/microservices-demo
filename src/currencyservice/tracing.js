"use strict";

const grpc = require('grpc');
const { NodeTracerProvider } = require("@opentelemetry/node");
const { SimpleSpanProcessor } = require("@opentelemetry/tracing");
const { CollectorTraceExporter } = require("@opentelemetry/exporter-collector-grpc");
const { GrpcInstrumentation } = require('@opentelemetry/instrumentation-grpc');
const { HttpInstrumentation } = require('@opentelemetry/instrumentation-http');
const { registerInstrumentations } = require('@opentelemetry/instrumentation');
const { ResourceAttributes } = require('@opentelemetry/semantic-conventions');
const { Resource } = require('@opentelemetry/resources');
const provider = new NodeTracerProvider( {
  resource: new Resource( {
    [ResourceAttributes.SERVICE_NAME]: process.env.SERVICE_NAME,
    [ 'ip' ]: process.env.POD_IP,
  } )
} );



const collectorOptions = {
	url: process.env.OTEL_EXPORTER_OTLP_ENDPOINT
  };

provider.addSpanProcessor(
  new SimpleSpanProcessor(
    new CollectorTraceExporter(collectorOptions)
  )
);

provider.register();

registerInstrumentations({
  tracerProvider: provider,
  instrumentations: [
    new GrpcInstrumentation(),
    new HttpInstrumentation()
  ]
});
console.log("tracing initialized");