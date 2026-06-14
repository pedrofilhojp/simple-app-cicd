FROM golang:1.24-alpine AS builder

WORKDIR /app

COPY go.mod .
COPY main.go .

RUN CGO_ENABLED=0 GOOS=linux go build -o app .

FROM alpine:3.22

RUN apk add --no-cache ca-certificates

WORKDIR /

COPY --from=builder /app/app .

EXPOSE 8080

CMD ["./app"]
