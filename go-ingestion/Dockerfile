FROM golang:1.26.5-alpine3.23@sha256:622e56dbc11a8cfe87cafa2331e9a201877271cbff918af53d3be315f3da88cc AS build
WORKDIR /src
ENV CGO_ENABLED=0 GOTOOLCHAIN=local
COPY go-ingestion/go.mod ./
COPY go-ingestion/cmd ./cmd
COPY go-ingestion/internal ./internal
RUN go build -trimpath -ldflags="-s -w" -o /out/go-ingestion ./cmd/server

FROM alpine:3.23.4@sha256:5b10f432ef3da1b8d4c7eb6c487f2f5a8f096bc91145e68878dd4a5019afde11 AS runtime
RUN addgroup -S -g 10001 app && adduser -S -D -H -u 10001 -G app app \
    && command -v wget
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=build /out/go-ingestion /usr/local/bin/go-ingestion
USER 10001:10001
EXPOSE 8080
HEALTHCHECK --interval=5s --timeout=3s --start-period=10s --retries=12 \
    CMD wget -q -O /dev/null -T 2 http://127.0.0.1:8080/health || exit 1
ENTRYPOINT ["/usr/local/bin/go-ingestion"]