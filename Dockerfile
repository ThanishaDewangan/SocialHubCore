FROM golang:1.24-alpine AS builder

WORKDIR /app

RUN apk add --no-cache git

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN go build -o /app/api cmd/api/main.go
RUN go build -o /app/worker cmd/worker/main.go

FROM alpine:latest AS api

RUN apk add --no-cache ca-certificates

WORKDIR /app

COPY --from=builder /app/api .

EXPOSE 5000

CMD ["./api"]

FROM alpine:latest AS worker

RUN apk add --no-cache ca-certificates

WORKDIR /app

COPY --from=builder /app/worker .

CMD ["./worker"]
