FROM node:22-alpine AS web-build

WORKDIR /src/web
RUN corepack enable && corepack prepare pnpm@10.30.3 --activate

COPY web/package.json web/pnpm-lock.yaml web/pnpm-workspace.yaml ./
RUN pnpm install --frozen-lockfile

COPY web/ ./
RUN pnpm build

FROM golang:1.25-bookworm AS server-build

ARG GOPROXY=https://proxy.golang.org,direct
ENV GOPROXY=${GOPROXY}

WORKDIR /src/server
COPY server/go.mod server/go.sum ./
RUN go mod download

COPY server/ ./
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/knowbr .

FROM alpine:3.22

WORKDIR /app
RUN apk add --no-cache ca-certificates wget \
    && addgroup -S knowbr \
    && adduser -S -G knowbr knowbr \
    && mkdir -p /app/data \
    && chown knowbr:knowbr /app/data

COPY --from=server-build /out/knowbr /app/knowbr
COPY --from=web-build /src/web/dist /app/web/dist

ENV ZXQ_PORT=8080 \
    ZXQ_DB_PATH=/app/data/knowbr.db \
    ZXQ_WEB_DIST=/app/web/dist

VOLUME ["/app/data"]
EXPOSE 8080
HEALTHCHECK --interval=10s --timeout=3s --start-period=5s --retries=5 \
    CMD wget --quiet --tries=1 --output-document=/dev/null http://127.0.0.1:8080/api/health || exit 1

USER knowbr
ENTRYPOINT ["/app/knowbr"]
