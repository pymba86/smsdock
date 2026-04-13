FROM node:22-alpine AS web-build

WORKDIR /src

RUN corepack enable

COPY package.json pnpm-lock.yaml pnpm-workspace.yaml ./
COPY packages/shared ./packages/shared
COPY packages/web ./packages/web

RUN pnpm install --frozen-lockfile --store-dir /tmp/pnpm-store
RUN pnpm --filter @smsdock/shared build
RUN pnpm --filter @smsdock/web build

FROM golang:1.23-alpine AS build

WORKDIR /src/packages/api

RUN apk add --no-cache ca-certificates git

COPY packages/api/go.mod packages/api/go.sum ./
RUN go mod download

COPY packages/api ./
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/smsdock-api ./cmd/api

FROM alpine:3.21

RUN apk add --no-cache ca-certificates

WORKDIR /app

COPY --from=build /out/smsdock-api /usr/local/bin/smsdock-api
COPY --from=web-build /src/packages/web/dist /app/web

ENV SMSDOCK_HTTP_ADDR=:8080
ENV SMSDOCK_DB_PATH=/var/lib/smsdock/smsdock.db
ENV SMSDOCK_WEB_DIST=/app/web
ENV SMSDOCK_DEVICE_GLOBS=/dev/serial/by-id/*,/dev/ttyUSB*

VOLUME ["/var/lib/smsdock"]

EXPOSE 8080

CMD ["smsdock-api"]
