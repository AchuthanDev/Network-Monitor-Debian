FROM alpine:3.22

RUN apk add --no-cache \
  busybox-extras \
  curl \
  iproute2 \
  nftables \
  socat

WORKDIR /src
COPY tests/gateway/simulate_gateway.sh /usr/local/bin/simulate_gateway.sh
ENTRYPOINT ["/usr/local/bin/simulate_gateway.sh"]
