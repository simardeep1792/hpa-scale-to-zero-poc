FROM golang:1.26 AS build
WORKDIR /src
COPY go.mod ./
COPY cmd ./cmd
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags='-s -w' -o /out/queue-lag-exporter ./cmd/queue-lag-exporter

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/queue-lag-exporter /queue-lag-exporter
USER 65532:65532
ENTRYPOINT ["/queue-lag-exporter"]
