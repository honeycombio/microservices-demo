#! bin/sh
java -jar /opt/otel/opentelemetry-jmx-metrics.jar -config session.properties &
/app/build/install/msdemo/bin/AdService