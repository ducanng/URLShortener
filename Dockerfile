<<<<<<< HEAD
#Build stage
FROM golang:1.16-alpine AS builder
WORKDIR /app

COPY go.mod .
COPY go.sum .
RUN go mod download

COPY . .

RUN go build -o main main.go

#Run stage
FROM alpine:latest
WORKDIR /app
COPY --from=builder /app/main .

EXPOSE 8080
CMD ["/app/main"]
=======
FROM golang:1.18.4-alpine3.16 AS builder
RUN apk add --no-cache git
WORKDIR /app


COPY go.mod .
COPY go.sum .
RUN go mod download
COPY . .

RUN go build -o bin/main main.go

FROM alpine:3.9
WORKDIR /app

COPY --from=builder /app/bin/main .

EXPOSE 8080
ENTRYPOINT ["./main"]
>>>>>>> 65b11c0454d2c48ca6981909bce446dbcc75b0fa
