#!/bin/sh
set -eu

for module in \
  features/network-usage \
  features/gateway \
  apps/collector \
  apps/backend
do
  echo "==> go test ./$module/..."
  (cd "$module" && go test ./...)
done
