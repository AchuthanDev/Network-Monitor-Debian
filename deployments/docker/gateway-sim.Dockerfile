FROM golang:1.23.10-alpine

RUN apk add --no-cache \
  busybox-extras \
  curl \
  iproute2 \
  nftables \
  socat

WORKDIR /src
COPY go.work ./
COPY features/gateway ./features/gateway
COPY features/network-usage ./features/network-usage
COPY features/traffic-classification ./features/traffic-classification
COPY apps/backend ./apps/backend
COPY apps/collector ./apps/collector
COPY apps/gatewayctl ./apps/gatewayctl
COPY tests/gateway/simulate_gateway.sh /usr/local/bin/simulate_gateway.sh

ENTRYPOINT ["/usr/local/bin/simulate_gateway.sh"]
