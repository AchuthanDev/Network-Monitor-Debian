#!/bin/sh
set -eu

for module in \
  features/network-usage \
  features/traffic-classification \
  features/gateway \
  apps/collector \
  apps/backend \
  apps/gatewayctl
do
  echo "==> go test ./$module/..."
  (cd "$module" && go test ./...)
done
