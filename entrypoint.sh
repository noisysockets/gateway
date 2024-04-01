#!/bin/bash
set -eum

cleanup () {
    wg-quick down wg0
    exit 0
}
trap cleanup SIGTERM SIGINT SIGQUIT

# Bring up WireGuard.
echo 'Creating WireGuard Interface ...'

wg-quick up /etc/wireguard/wg0.conf

MAX_ATTEMPTS=10

# Make sure WireGuard is up.
count=0
while ! wg show wg0 > /dev/null; do
    count=$((count+1))
    if [ "$count" -ge "$MAX_ATTEMPTS" ]; then
        echo "WireGuard failed to come up after $MAX_ATTEMPTS attempts."
        cleanup
    fi

    sleep 1
done

# Run a dnsmasq DNS forwarder.
echo 'Starting dnsmasq DNS forwarder ...'

dnsmasq -i wg0 -k