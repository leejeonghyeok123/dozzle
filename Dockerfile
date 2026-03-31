# Build assets
FROM --platform=$BUILDPLATFORM node:25.8.1-alpine AS node

RUN npm install -g --force corepack && corepack enable

WORKDIR /build

# Install dependencies from lock file
COPY pnpm-*.yaml ./
RUN pnpm fetch --ignore-scripts --no-optional

# Copy package.json and install dependencies
COPY package.json ./
RUN pnpm install --offline --ignore-scripts --no-optional

# Copy assets and translations to build
COPY vite.config.ts tsconfig.json .prettierrc.cjs .npmrc ./
COPY assets ./assets
COPY locales ./locales
COPY public ./public

# Build assets
RUN pnpm build

FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS builder

RUN apk add --no-cache ca-certificates && mkdir /dozzle

WORKDIR /dozzle

# Copy go mod files
COPY go.* ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download

# Copy all other files
COPY internal ./internal
COPY types ./types
COPY main.go ./
COPY protos ./protos
COPY certs/dozzle_key.pem certs/dozzle_cert.pem ./
RUN cp dozzle_key.pem shared_key.pem && cp dozzle_cert.pem shared_cert.pem

# Copy assets built with node
COPY --from=node /build/dist ./dist

# Args
ARG TAG=dev
ARG TARGETOS TARGETARCH

# Build binary
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build \
  GOOS=$TARGETOS GOARCH=$TARGETARCH CGO_ENABLED=0 go build -ldflags "-s -w -X github.com/amir20/dozzle/internal/support/cli.Version=$TAG" -o dozzle

RUN mkdir /data

FROM alpine:3.22

RUN apk add --no-cache ca-certificates git docker-cli docker-cli-compose

COPY --from=builder /data /data
COPY --from=builder /tmp /tmp
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=builder /dozzle/dozzle /dozzle

EXPOSE 8080

ENTRYPOINT ["/dozzle"]
