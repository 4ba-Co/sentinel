FROM golang:1.25-alpine AS builder

WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o sentinel .

FROM alpine:latest
COPY --from=builder /build/sentinel /usr/local/bin/sentinel

ENTRYPOINT ["sentinel", "--"]
CMD ["sh", "-c", "echo 'Hello from container'"]
