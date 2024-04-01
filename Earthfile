VERSION 0.7
FROM golang:1.22-bookworm
WORKDIR /workspace

all:
  ARG VERSION=latest
  BUILD --platform=linux/amd64 --platform=linux/arm64 +docker

docker:
  FROM debian:bookworm-slim
  RUN apt update
  RUN apt install -y iptables iproute2 dnsmasq
  COPY +wireguard-go/wireguard-go /usr/bin/wireguard-go
  COPY +wireguard-tools/wg /usr/bin/
  COPY +wireguard-tools/wg-quick /usr/bin/
  RUN umask 077 && mkdir -p /etc/wireguard
  COPY entrypoint.sh /entrypoint.sh
  RUN chmod +x /entrypoint.sh
  ENTRYPOINT ["/entrypoint.sh"]
  ARG VERSION=latest
  SAVE IMAGE --push ghcr.io/noisysockets/gateway:${VERSION}
  SAVE IMAGE --push ghcr.io/noisysockets/gateway:latest

tidy:
  LOCALLY
  WORKDIR tests
  RUN go mod tidy
  RUN go fmt ./...

lint:
  FROM golangci/golangci-lint:v1.57.2
  RUN apt update
  RUN apt install -y shellcheck
  COPY entrypoint.sh .
  RUN shellcheck entrypoint.sh
  COPY tests/ tests
  WORKDIR tests
  RUN go mod download
  RUN golangci-lint run --timeout 5m .

test:
  FROM +tools
  COPY tests/go.mod tests/go.sum .
  RUN go mod download
  COPY tests/ tests
  ARG VERSION=latest-dev
  WITH DOCKER --load ghcr.io/noisysockets/gateway:${VERSION}=(+docker --VERSION=${VERSION}) --allow-privileged
    RUN go run tests/main.go
  END

wireguard-go:
  GIT CLONE --branch=0.0.20230223 https://git.zx2c4.com/wireguard-go.git .
  RUN make -j$(nproc)
  SAVE ARTIFACT wireguard-go

wireguard-tools:
  GIT CLONE --branch=v1.0.20210914 https://git.zx2c4.com/wireguard-tools.git .
  WORKDIR src
  RUN make -j$(nproc)
  SAVE ARTIFACT wg
  SAVE ARTIFACT wg-quick/linux.bash wg-quick

tools:
  RUN apt update
  RUN apt install -y ca-certificates curl jq
  RUN curl -fsSL https://get.docker.com | bash