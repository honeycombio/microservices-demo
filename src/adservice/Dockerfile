FROM openjdk:11-slim as builder

WORKDIR /app

COPY ["build.gradle", "gradlew", "./"]
COPY gradle gradle
COPY startup.sh startup.sh
COPY session.properties session.properties
RUN chmod +x gradlew
RUN ./gradlew downloadRepos

COPY . .
RUN chmod +x gradlew
RUN ./gradlew installDist

FROM openjdk:11-slim

RUN apt-get -y update && apt-get install -qqy \
    wget \
    && rm -rf /var/lib/apt/lists/*

RUN mkdir -p /opt/otel && \
    wget -q -O opt/otel/javaagent.jar https://github.com/honeycombio/honeycomb-opentelemetry-java/releases/download/v0.10.0/honeycomb-opentelemetry-javaagent-0.10.0.jar && \
    wget -q -O opt/otel/opentelemetry-jmx-metrics.jar https://github.com/open-telemetry/opentelemetry-java-contrib/releases/download/v1.12.0/opentelemetry-jmx-metrics.jar

WORKDIR /app
COPY --from=builder /app .
RUN chmod +x startup.sh

EXPOSE 9555
ENTRYPOINT ["sh", "/app/startup.sh"]
