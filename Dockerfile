# 构建阶段
FROM golang:1.24-alpine AS builder

WORKDIR /app

COPY go.mod ./
COPY go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o go-shorten ./cmd/main.go

# 运行阶段
FROM alpine:latest

WORKDIR /app

COPY --from=builder /app/go-shorten .
RUN mkdir -p /app/data

EXPOSE 5768

ENTRYPOINT ["./go-shorten"]