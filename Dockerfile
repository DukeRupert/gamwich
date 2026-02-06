# Stage 1 — Build
FROM golang:1.25-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -o /app/gamwich ./cmd/gamwich

# Stage 2 — Runtime
FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

COPY --from=build /app/gamwich .
COPY --from=build /src/web/ ./web/

EXPOSE 8080

ENV GAMWICH_DB_PATH=/data/gamwich.db

ENTRYPOINT ["./gamwich"]
