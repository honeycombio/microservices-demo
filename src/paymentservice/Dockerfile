FROM node:22.6-alpine AS base

FROM base as builder

RUN apk add --update --no-cache \
    python3 \
    make \
    g++ 

WORKDIR /usr/src/app

COPY package*.json ./

RUN npm install --omit=dev

FROM base

WORKDIR /usr/src/app

COPY --from=builder /usr/src/app/node_modules ./node_modules

COPY . .

EXPOSE 50051

ENTRYPOINT [ "node", "-r", "./tracing.js", "index.js" ]
