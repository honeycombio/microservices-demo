FROM mcr.microsoft.com/dotnet/sdk:6.0.300 AS builder

WORKDIR /usr/src/app/

COPY . .

RUN \
  RUNTIME_IDENTIIFER=linux-musl-x64; \
  if [ "$(uname -m)" = "aarch64" ]; then RUNTIME_IDENTIIFER=linux-musl-arm64; fi; \
  dotnet restore ./cartservice.csproj -r $RUNTIME_IDENTIIFER && \
  dotnet publish ./cartservice.csproj -p:PublishSingleFile=true -r $RUNTIME_IDENTIIFER --self-contained true -p:PublishTrimmed=True -p:TrimMode=Link -c Release -o /cartservice --no-restore

# -----------------------------------------------------------------------------

# https://mcr.microsoft.com/v2/dotnet/runtime-deps/tags/list
FROM mcr.microsoft.com/dotnet/runtime-deps:6.0.5-alpine3.15

WORKDIR /usr/src/app/
COPY --from=builder /cartservice/ ./

ENV ASPNETCORE_URLS http://*:7070

ENTRYPOINT [ "./cartservice" ]
