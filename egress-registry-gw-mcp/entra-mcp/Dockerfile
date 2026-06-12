FROM golang:1.23-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o entra-mcp .

FROM alpine:latest
WORKDIR /app
COPY --from=builder /app/entra-mcp .
EXPOSE 8080
CMD ["./entra-mcp"]
