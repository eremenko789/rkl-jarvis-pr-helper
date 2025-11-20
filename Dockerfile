FROM golang:1.25 AS builder

WORKDIR /src
COPY go.mod go.sum ./
RUN date && go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o /out/webhook-service ./cmd/webhook-service

FROM ubuntu:noble

WORKDIR /app

COPY --from=builder /out/webhook-service /usr/local/bin/webhook-service
COPY config.example.yaml /etc/webhook/config.yaml
# COPY certs/ /usr/local/share/ca-certificates
# RUN update-ca-certificates

ENV CONFIG_FILE=/etc/webhook/config.yaml

EXPOSE 8080

ENTRYPOINT ["webhook-service", "run", "-config", "/etc/webhook/config.yaml", "--debug"]
