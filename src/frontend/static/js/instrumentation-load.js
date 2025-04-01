import { HoneycombWebSDK } from '@honeycombio/opentelemetry-web';
import { getWebAutoInstrumentations } from '@opentelemetry/auto-instrumentations-web';

const configDefaults = {
  ignoreNetworkEvents: false,
  propagateTraceHeaderCorsUrls: [
    /.+/g, // Regex to match your backend URLs. Update to the domains you wish to include.
  ]
}

const sdk = new HoneycombWebSDK({
  endpoint: "https://microservicescollector.honeydemo.io:443/v1/traces", // Send to EU instance of Honeycomb. Defaults to sending to US instance.
  debug: true, // Set to false for production environment.
  apiKey: '', // Replace with your Honeycomb Ingest API Key. Leave this value as '' in order for any CORS to work.
  serviceName: 'browser', // Replace with your application name. Honeycomb uses this string to find your dataset when we receive your data. When no matching dataset exists, we create a new one with this name if your API Key has the appropriate permissions.
  instrumentations: [getWebAutoInstrumentations({
    // Loads custom configuration for xml-http-request instrumentation.
    '@opentelemetry/instrumentation-xml-http-request': configDefaults,
    '@opentelemetry/instrumentation-fetch': configDefaults,
    '@opentelemetry/instrumentation-document-load': configDefaults,
    '@opentelemetry/instrumentation-user-interaction': configDefaults,
  })],
});
sdk.start();