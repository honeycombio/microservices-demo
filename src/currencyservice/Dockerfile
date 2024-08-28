FROM node:22.6-alpine AS base

FROM base AS builder

RUN apk add --no-cache \
    python3 \
    g++ \
    make

WORKDIR /usr/src/app

COPY package*.json ./

RUN npm install --omit=dev

FROM base

WORKDIR /usr/src/app

COPY --from=builder /usr/src/app/node_modules ./node_modules

COPY . .

EXPOSE 7000

ENTRYPOINT [ "node", "-r", "./tracing.js", "server.js" ]
