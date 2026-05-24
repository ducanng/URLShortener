module:
	go mod init github.com/ducanng/URLShortener
	go mod tidy
gen-urlshortener:
	protoc -I . -I third_party \
		--go_out=. --go_opt=module=github.com/ducanng/URLShortener \
		--go-grpc_out=. --go-grpc_opt=module=github.com/ducanng/URLShortener \
		--grpc-gateway_out=. --grpc-gateway_opt=module=github.com/ducanng/URLShortener \
		proto/urlshortener.proto
go-build:
	go build -o bin/main.exe main.go
swag:
	swag init
dockerimage:
	docker-compose up -d