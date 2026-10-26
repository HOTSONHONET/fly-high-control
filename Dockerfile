FROM golang:1.25-bookworm AS builder

WORKDIR /src

COPY go.mod ./
RUN go mod download

COPY . .
RUN go mod tidy
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/fly-high-control ./cmd/fly-high-control

FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
    alsa-utils \
    ca-certificates \
    espeak-ng \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

COPY --from=builder /out/fly-high-control /usr/local/bin/fly-high-control

ENTRYPOINT ["/usr/local/bin/fly-high-control"]
