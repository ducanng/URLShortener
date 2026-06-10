FROM golang:1.26.3-alpine3.23 AS builder
RUN apk add --no-cache git
WORKDIR /app


COPY go.mod .
COPY go.sum .
RUN go mod download
COPY . .

RUN go build -o bin/main ./
RUN go build -o bin/migrate ./cmd/migrate

FROM alpine:3.23
WORKDIR /app

COPY --from=builder /app/bin/main /app/main
COPY --from=builder /app/bin/migrate /app/migrate

EXPOSE 8080
ENTRYPOINT ["/app/main"]
