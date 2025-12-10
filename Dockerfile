FROM golang:1.23-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o chatroom .

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/chatroom .
COPY --from=builder /app/config.toml ./config.toml
CMD ["./chatroom"]