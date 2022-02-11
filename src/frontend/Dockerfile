FROM golang:1.17-alpine as builder
RUN apk add --no-cache ca-certificates git
WORKDIR /src

# restore dependencies
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o /go/bin/frontend .

FROM alpine as release
RUN apk add --no-cache ca-certificates \
    busybox-extras net-tools bind-tools
WORKDIR /frontend
COPY --from=builder /go/bin/frontend /frontend/server
COPY ./templates ./templates
COPY ./static ./static
COPY ./dist ./dist
EXPOSE 8080
ENTRYPOINT ["/frontend/server"]
