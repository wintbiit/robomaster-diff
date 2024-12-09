FROM golang:1.23-alpine as builder

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY . .

RUN go build -trimpath -ldflags "-s -w" -o /app/main cmd/cli/main.go

FROM alpine:3.12
ENV STORAGE_PATH=/app

RUN apk add --no-cache ca-certificates

WORKDIR /app

COPY --from=builder /app/main /usr/local/bin/robomaster-diff

VOLUME /app

CMD ["/usr/local/bin/robomaster-diff"]