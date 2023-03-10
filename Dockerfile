FROM golang:1.18.4-alpine3.16 AS builder
RUN apk add --no-cache git
WORKDIR /app


COPY go.mod .
COPY go.sum .
RUN go mod download
COPY . .

RUN go build -o bin/main ./cmd/main.go

FROM alpine:3.9
WORKDIR /app

COPY --from=builder /app/bin/main .

EXPOSE 8080
ENTRYPOINT ["./main"]
