FROM golang:1.26.3-alpine3.23 AS builder
RUN apk add --no-cache git
WORKDIR /app

COPY go.mod .
COPY go.sum .
RUN go mod tidy
COPY . .

RUN go build -o bin/main main.go

FROM alpine:3.23
WORKDIR /app

COPY --from=builder /app/bin/main .
COPY --from=builder /app/.env .
COPY --from=builder /app/templates ./templates

EXPOSE 8080
ENTRYPOINT ["./main"]
