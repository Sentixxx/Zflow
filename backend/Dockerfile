FROM golang:1.25-alpine AS builder
RUN apk add --no-cache gcc musl-dev
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=1 go build -trimpath -ldflags="-s -w" -o /zflow ./cmd/server

# Use alpine (not scratch) so wget is available for the healthcheck.
FROM alpine:3.21
RUN apk add --no-cache wget ca-certificates
COPY --from=builder /zflow /zflow
EXPOSE 8080
ENTRYPOINT ["/zflow"]
