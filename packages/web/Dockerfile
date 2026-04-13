FROM node:22-bookworm AS build

WORKDIR /src

RUN corepack enable

COPY package.json pnpm-lock.yaml pnpm-workspace.yaml ./
COPY packages/shared ./packages/shared
COPY packages/web ./packages/web

ARG VITE_API_BASE=
ENV VITE_API_BASE=${VITE_API_BASE}

RUN pnpm install --frozen-lockfile --store-dir /tmp/pnpm-store
RUN pnpm --filter @smsdock/shared build
RUN pnpm --filter @smsdock/web build

FROM nginx:1.27-alpine

COPY deploy/docker/nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=build /src/packages/web/dist /usr/share/nginx/html

EXPOSE 80

