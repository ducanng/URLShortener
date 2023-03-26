FROM golang:1.20-alpine3.16 AS builder
RUN apk add --no-cache git
WORKDIR /app

COPY go.mod .
COPY go.sum .
RUN go mod download
COPY . .

RUN go build -o bin/main main.go

FROM alpine:3.14
WORKDIR /app

COPY --from=builder /app/bin/main .
COPY --from=builder /app/.env .
COPY --from=builder /app/templates ./templates

EXPOSE 8080
ENTRYPOINT ["./main"]
