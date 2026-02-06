# Stage 1 — Build
FROM golang:1.25-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -o /app/gamwich ./cmd/gamwich

# Stage 2 — Runtime
FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata \
    && ARCH=$(uname -m) \
    && if [ "$ARCH" = "x86_64" ]; then CF_ARCH="amd64"; \
       elif [ "$ARCH" = "aarch64" ]; then CF_ARCH="arm64"; \
       else CF_ARCH="amd64"; fi \
    && wget -q -O /usr/local/bin/cloudflared \
       "https://github.com/cloudflare/cloudflared/releases/latest/download/cloudflared-linux-${CF_ARCH}" \
    && chmod +x /usr/local/bin/cloudflared

WORKDIR /app

COPY --from=build /app/gamwich .
COPY --from=build /src/web/ ./web/

EXPOSE 8080

ENV GAMWICH_DB_PATH=/data/gamwich.db

ENTRYPOINT ["./gamwich"]
