# syntax=docker/dockerfile:1

FROM golang:1.27-alpine3.24 AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download

COPY . .
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 go build -mod=readonly -trimpath -ldflags="-s -w" \
    -o /out/sayrud-server ./cmd/sayrud-server

FROM alpine:3.24

RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -S -g 10001 app \
    && adduser -S -D -H -u 10001 -G app app

WORKDIR /home/app
COPY --from=build --chmod=0555 /out/sayrud-server ./sayrud-server

# Mount configuration at /home/app/config/sayrud.yaml, readable by UID 10001.
USER 10001:10001

EXPOSE 2830
ENTRYPOINT ["/home/app/sayrud-server"]
