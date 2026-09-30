# syntax=docker/dockerfile:1

FROM golang:1.26-alpine AS builder

ARG TARGETOS
ARG TARGETARCH

WORKDIR /src

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download

COPY . .
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -trimpath -ldflags="-s -w" -o /out/mini-lambda .

FROM alpine:3.22

# Worker shells out to `docker run`, so the CLI must be present.
RUN apk add --no-cache ca-certificates tzdata docker-cli

WORKDIR /app

COPY --from=builder /out/mini-lambda /app/mini-lambda
COPY migrations /app/migrations

ENV STORAGE_PROVIDER=minio \
    MINIO_ENDPOINT="" \
    MINIO_ACCESS_KEY="" \
    MINIO_SECRET_KEY="" \
    MINIO_BUCKET="" \
    MINIO_REGION="" \
    SUPABASE_URL="" \
    SUPABASE_KEY="" \
    SUPABASE_BUCKET="" \
    DB_URL="" \
    MAX_DOCKER_MEMORY_MB=2048 \
    MAX_DOCKER_CPUS=4

EXPOSE 3000

ENTRYPOINT ["/app/mini-lambda"]
