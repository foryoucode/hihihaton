FROM golang:1.23-alpine3.21 AS builder
WORKDIR /app
COPY .. .
RUN go mod download
RUN go build -o webapp cmd/main.go

FROM alpine:latest
WORKDIR /root/
COPY --from=builder /app/webapp .
COPY --from=builder /app/scripts /scripts
CMD ["./webapp"]
