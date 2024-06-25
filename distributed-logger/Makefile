
all: clean client server

client: app/client/client.go
	go build -o bin/client app/client/client.go

server: app/server/server.go
	go build -o bin/server app/server/server.go

clean:
	rm -rf bin/*
	rm -rf analysis/data.log

.PHONY: all client server clean