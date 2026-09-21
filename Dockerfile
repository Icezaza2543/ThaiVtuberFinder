FROM golang:1.26-bookworm AS build
RUN apt-get update && apt-get install -y --no-install-recommends libsqlite3-dev pkg-config && rm -rf /var/lib/apt/lists/*
WORKDIR /src
COPY go.mod ./
COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=1 go test ./... && CGO_ENABLED=1 go build -trimpath -ldflags="-s -w" -o /out/finder ./cmd/finder

FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates libsqlite3-0 && rm -rf /var/lib/apt/lists/* \
    && useradd --uid 10001 --create-home --shell /usr/sbin/nologin finder \
    && mkdir -p /app/data /app/config && chown -R finder:finder /app
COPY --from=build /out/finder /usr/local/bin/finder
COPY config /app/config
USER finder
WORKDIR /app
ENV FINDER_DB=/app/data/finder.sqlite HEALTH_ADDRESS=0.0.0.0:8080
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=5s --start-period=20s --retries=3 CMD ["/usr/local/bin/finder", "healthcheck"]
ENTRYPOINT ["/usr/local/bin/finder"]
CMD ["worker", "-config", "/app/config/finder.json"]
