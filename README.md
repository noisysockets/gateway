# gateway

A WireGuard-Go Docker Image With An Embedded DNS Forwarder.

## Usage

Make sure you have a WireGuard configuration file named `wg0.conf` in the current directory, and then run the following command.

This assumes you've configured WireGuard to listen on port 51820, change the port number if you've configured it differently.

```bash
docker run -d --rm \
  --cap-add=NET_ADMIN \
  --sysctl=net.ipv4.ip_forward=1 --sysctl=net.ipv4.conf.all.src_valid_mark=1 \
  -p51820:51820/udp \
  -v /dev/net/tun:/dev/net/tun \
  -v $(pwd)/wg0.conf:/etc/wireguard/wg0.conf:ro \
  ghcr.io/noisysockets/gateway:latest
```