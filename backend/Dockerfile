FROM golang:1.24-alpine AS builder

WORKDIR /app

RUN apk add --no-cache git ca-certificates tzdata

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /app/bin/api ./cmd/api
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /app/bin/seed ./cmd/seed

FROM alpine:3.19

WORKDIR /app

RUN apk add --no-cache ca-certificates tzdata curl
ENV TZ=Asia/Ho_Chi_Minh

COPY --from=builder /app/bin/api /app/api
COPY --from=builder /app/bin/seed /app/seed
COPY docs /app/docs
COPY .env.example /app/.env

# Create uploads directory with write permissions
RUN mkdir -p /app/uploads && chmod 777 /app/uploads

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD curl -f http://localhost:8080/health || exit 1

ENTRYPOINT ["/app/api"]
