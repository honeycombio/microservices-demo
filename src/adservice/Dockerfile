FROM openjdk:11-slim as builder

WORKDIR /app

COPY ["build.gradle", "gradlew", "./"]
COPY gradle gradle
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
    wget -q -O opt/otel/javaagent.jar https://github.com/honeycombio/honeycomb-opentelemetry-java/releases/download/v0.10.0/honeycomb-opentelemetry-javaagent-0.10.0.jar
WORKDIR /app
COPY --from=builder /app .

EXPOSE 9555
ENTRYPOINT ["/app/build/install/msdemo/bin/AdService"]
