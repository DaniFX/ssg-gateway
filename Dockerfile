FROM golang:1.25-alpine AS builder

WORKDIR /app

RUN apk add --no-cache git

COPY . .
RUN ls -la /app
RUN go mod download

RUN CGO_ENABLED=0 GOOS=linux go build -o /gateway ./cmd/gateway/main.go

FROM alpine:3.19

RUN apk add --no-cache ca-certificates

WORKDIR /app

COPY --from=builder /gateway .

EXPOSE 8080

CMD ["./gateway"]
