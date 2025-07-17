# syntax=docker/dockerfile:1

FROM golang:1.20-bullseye

WORKDIR /app

# GStreamerとビルド関連のツール、および tzdata をインストール
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
    tzdata \
    && rm -rf /var/lib/apt/lists/*

# タイムゾーンをJST（Asia/Tokyo）に設定
ENV TZ=Asia/Tokyo
RUN ln -fs /usr/share/zoneinfo/Asia/Tokyo /etc/localtime && \
    dpkg-reconfigure -f noninteractive tzdata

# Goの依存関係を解決
COPY go.mod .
RUN go mod download
COPY main.go .
RUN go mod tidy

# アプリケーションコードとその他ファイルをコピー
COPY . .

# アプリケーションをビルド
RUN go mod tidy
RUN go build -o sender main.go

# アプリケーション起動
CMD ["./sender"]
