FROM golang:1.17-alpine AS builder
WORKDIR /app/
ADD go.mod go.sum ./
RUN go mod download
ADD . .
RUN GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-w -s" -o speedtest-exporter main.go

FROM golang:1.17-alpine
WORKDIR /app/
COPY --from=builder /app/speedtest-exporter /app/speedtest-exporter
ENTRYPOINT ["/app/speedtest-exporter"]
