FROM golang:1.23.10-alpine AS build
WORKDIR /src
ENV GOWORK=off
COPY go.work ./
COPY apps/backend ./apps/backend
RUN cd apps/backend && go test ./...
RUN cd apps/backend && go build -o /out/network-monitor-api ./cmd/api

FROM alpine:3.22
RUN adduser -D -H -s /sbin/nologin appuser
COPY --from=build /out/network-monitor-api /bin/network-monitor-api
USER appuser
EXPOSE 8080
ENTRYPOINT ["/bin/network-monitor-api"]
