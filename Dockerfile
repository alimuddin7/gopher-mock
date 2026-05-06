# syntax=docker/dockerfile:1.4

FROM golang:1.25-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN go build -o mockserver main.go

# Runtime image
FROM alpine:latest

WORKDIR /app

COPY --from=builder /app/mockserver /app/
COPY --from=builder /app/templates/ /app/templates/
COPY --from=builder /app/static/ /app/static
# COPY config.json /app/

EXPOSE 8080

ENTRYPOINT ["./mockserver"]
