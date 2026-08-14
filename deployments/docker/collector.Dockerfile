FROM golang:1.23.10-alpine AS build
WORKDIR /src
ENV GOWORK=off
COPY go.work ./
COPY apps/collector ./apps/collector
COPY features/network-usage ./features/network-usage
RUN cd features/network-usage && go test ./...
RUN cd apps/collector && go test ./...
RUN cd apps/collector && go build -o /out/network-monitor-collector ./cmd/collector

FROM alpine:3.22
COPY --from=build /out/network-monitor-collector /bin/network-monitor-collector
EXPOSE 9091
ENTRYPOINT ["/bin/network-monitor-collector"]
