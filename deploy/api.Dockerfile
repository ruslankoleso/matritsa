# syntax=docker/dockerfile:1
# One Dockerfile for all Go services: docker build -f deploy/api.Dockerfile --build-arg CMD=api|mock-bki|mock-egrul .
FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG CMD=api
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/app ./cmd/${CMD}

FROM alpine:3.20
RUN adduser -D -u 10001 app && mkdir -p /data && chown app /data
USER app
COPY --from=build /out/app /app
ENV DATA_DIR=/data
ENTRYPOINT ["/app"]
