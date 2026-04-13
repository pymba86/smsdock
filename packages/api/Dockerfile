FROM golang:1.23-bookworm AS build

WORKDIR /src/packages/api

COPY packages/api/go.mod packages/api/go.sum ./
RUN go mod download

COPY packages/api ./
RUN CGO_ENABLED=0 go build -o /out/smsdock-api ./cmd/api

FROM debian:bookworm-slim

RUN apt-get update \
  && apt-get install -y --no-install-recommends ca-certificates wget \
  && rm -rf /var/lib/apt/lists/*

WORKDIR /app

COPY --from=build /out/smsdock-api /usr/local/bin/smsdock-api

ENV SMSDOCK_HTTP_ADDR=:8080
ENV SMSDOCK_DB_PATH=/var/lib/smsdock/smsdock.db
ENV SMSDOCK_DEVICE_GLOBS=/dev/serial/by-id/*,/dev/ttyUSB*

VOLUME ["/var/lib/smsdock"]

EXPOSE 8080

CMD ["smsdock-api"]
