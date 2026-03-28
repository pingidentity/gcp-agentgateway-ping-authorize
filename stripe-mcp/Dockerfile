FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o stripe-mcp .

FROM alpine:latest
WORKDIR /app
COPY --from=builder /app/stripe-mcp .
EXPOSE 8080
CMD ["./stripe-mcp"]
