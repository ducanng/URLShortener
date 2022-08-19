module:
	go mod init URLShortener-gRPC-Swagger
	go mod tidy
gen-urlshortener:
	protoc proto/urlshortener.proto --go-grpc_out=.
	protoc proto/urlshortener.proto --go_out=.
go-build:
	go build -o bin/main.exe main.go
go-swag:
	swag init