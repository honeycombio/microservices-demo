FROM amazoncorretto:23 as builder

WORKDIR /app

COPY ["build.gradle", "gradlew", "./"]
COPY gradle gradle
RUN chmod +x gradlew
RUN ./gradlew downloadRepos

COPY . .
RUN chmod +x gradlew
RUN ./gradlew installDist

FROM amazoncorretto:23

RUN yum install -y \
    wget \
    findutils \
    && yum clean all \
    && rm -rf /var/cache/yum

RUN mkdir -p /opt/otel && \
    wget -q -O /opt/otel/javaagent.jar https://github.com/open-telemetry/opentelemetry-java-instrumentation/releases/latest/download/opentelemetry-javaagent.jar

WORKDIR /app
COPY --from=builder /app .

EXPOSE 9555
ENTRYPOINT ["/app/build/install/msdemo/bin/AdService"]
