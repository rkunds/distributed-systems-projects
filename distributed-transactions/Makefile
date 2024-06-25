all: client server

proto: proto/*.proto
	protoc --go-grpc_out=proto --go_out=proto proto/*.proto

client: app/client/client.go
	go build -o client app/client/client.go
	go build -o mp3/client app/client/client.go

server: app/server/cmap.go app/server/account.go app/server/transaction.go app/server/bank_service.go app/server/server.go
	go build -o server app/server/cmap.go app/server/account.go app/server/transaction.go app/server/bank_service.go app/server/server.go
	go build -o mp3/server app/server/cmap.go app/server/account.go app/server/transaction.go app/server/bank_service.go app/server/server.go

clean:
	rm -rf client server
