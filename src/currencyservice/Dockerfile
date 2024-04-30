FROM node:22-bullseye as base

FROM base as builder

RUN apt-get update && apt-get install -y \
    python3 \
    g++ \
    make

WORKDIR /usr/src/app

COPY package*.json ./

# RUN npm config set registry https://registry.npmjs.org/
RUN npm ci

FROM base

RUN GRPC_HEALTH_PROBE_VERSION=v0.3.6 && \
    wget -qO/bin/grpc_health_probe https://github.com/grpc-ecosystem/grpc-health-probe/releases/download/${GRPC_HEALTH_PROBE_VERSION}/grpc_health_probe-linux-amd64 && \
    chmod +x /bin/grpc_health_probe

WORKDIR /usr/src/app

COPY --from=builder /usr/src/app/node_modules ./node_modules

COPY . .

EXPOSE 7000

ENTRYPOINT [ "node", "-r", "./tracing.js", "server.js" ]
