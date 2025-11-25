FROM golang:1.25-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /ironbuckets ./cmd/server

FROM alpine:latest
RUN apk --no-cache add ca-certificates wget
WORKDIR /root/
COPY --from=builder /ironbuckets .
COPY views ./views
EXPOSE 8080
CMD ["./ironbuckets"]
