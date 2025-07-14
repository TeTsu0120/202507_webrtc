# syntax=docker/dockerfile:1

FROM golang:1.20 AS builder

WORKDIR /app


COPY go.mod .
COPY main.go .
RUN go mod tidy

COPY . .

RUN go mod tidy

RUN go build -o signaling-server main.go

FROM debian:bookworm

WORKDIR /app

COPY --from=builder /app/signaling-server .

EXPOSE 8080

CMD ["./signaling-server"]
