# syntax=docker/dockerfile:1

FROM golang:1.20-buster

WORKDIR /app

# 必要パッケージをインストール（GStreamer対応）
RUN apt update && apt install -y \
    gstreamer1.0-tools \
    gstreamer1.0-plugins-base \
    gstreamer1.0-plugins-good \
    gstreamer1.0-plugins-bad \
    gstreamer1.0-plugins-ugly \
    gstreamer1.0-libav \
    libgstreamer1.0-dev \
    libgstreamer-plugins-base1.0-dev \
    build-essential \
    pkg-config && \
    rm -rf /var/lib/apt/lists/*

COPY go.mod .
COPY main.go .
RUN go mod tidy
RUN go build -o sender main.go

CMD ["./sender"]
