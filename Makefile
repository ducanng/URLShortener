module:
	go mod init URLShortener-gRPC-Swagger
	go mod tidy
gen-urlshortener:
	protoc proto/urlshortener.proto --go-grpc_out=.
	protoc proto/urlshortener.proto --go_out=.
run-server:
	go run server/server.go
run-client:
	go run main.go
go-build:
	go build -o bin/server.exe server/server.go
	go build -o bin/client.exe main.go