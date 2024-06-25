all: clean proto node logger

clean:
	rm -rf bin/*
	rm -rf logs/*

proto: proto/message.proto
	protoc --go-grpc_out=proto --go_out=proto proto/message.proto

node: main.go
	go build -o bin/node main.go

logger: logger_server.go
	go build -o bin/logger_server logger_server.go

.PHONY: all clean proto node
