FROM golang:1.17-alpine AS builder
RUN apk add --no-cache ca-certificates git

WORKDIR /src
# restore dependencies
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o /storageservice .

FROM alpine AS release
RUN apk add --no-cache ca-certificates
RUN GRPC_HEALTH_PROBE_VERSION=v0.3.6 && \
    wget -qO/bin/grpc_health_probe https://github.com/grpc-ecosystem/grpc-health-probe/releases/download/${GRPC_HEALTH_PROBE_VERSION}/grpc_health_probe-linux-amd64 && \
    chmod +x /bin/grpc_health_probe
WORKDIR /storageservice
COPY --from=builder /storageservice ./server
EXPOSE 6381
ENTRYPOINT ["/storageservice/server"]

