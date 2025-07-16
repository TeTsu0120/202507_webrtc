# syntax=docker/dockerfile:1

FROM golang:1.20-bullseye

WORKDIR /app

# GStreamerとビルド関連のツールをインストール
RUN apt update && apt install -y \
    gstreamer1.0-tools \
    gstreamer1.0-plugins-base \
    gstreamer1.0-plugins-good \
    gstreamer1.0-plugins-bad \
    gstreamer1.0-plugins-ugly \
    gstreamer1.0-libav \
    libgstreamer1.0-dev \
    libgstreamer-plugins-base1.0-dev \
    libgstreamer-plugins-bad1.0-dev \
    build-essential \
    pkg-config \
    ca-certificates \
    iproute2 \
    && rm -rf /var/lib/apt/lists/*

# Goの依存関係を解決
COPY go.mod .
RUN go mod download
COPY main.go .
RUN go mod tidy

# アプリケーションコードと動画ファイルをコピー
COPY . .

# アプリケーションをビルド

RUN go mod tidy
RUN go build -o sender main.go

# 動画ファイル（検証用）を /app に配置
# （もし . にすでに video.mp4 があるなら上の COPY . . でOK）
# COPY video.mp4 /app/video.mp4

COPY entry.sh /app/entry.sh
RUN chmod +x /app/entry.sh

ENTRYPOINT ["/app/entry.sh"]