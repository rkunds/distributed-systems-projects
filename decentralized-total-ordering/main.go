package main

import (
	"CS425MP1/multicast"
	"bufio"
	"fmt"
	"os"
	"strconv"

	c "CS425MP1/common"
	pb "CS425MP1/proto"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func main() {
	debug := true
	args := os.Args
	if len(args) != 3 {
		fmt.Println("Usage: ./node <node ID> <config file>")
		return
	}

	nodeID := args[1]
	configFile := args[2]

	peerInfo := ReadConfigFile(configFile)

	transactionChan := make(chan c.ChanStruct, 100)

	server := multicast.NewMulticast(nodeID, peerInfo,
		transactionChan)

	if debug {
		debug_addr := "localhost:8888"
		conn, err := grpc.Dial(debug_addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			fmt.Println("Error dialing debug server")
		}

		log_client := pb.NewMessageServiceClient(conn)
		server.SetDebugLogger(&log_client)
	}
	go server.Start()

	reader := bufio.NewReader(os.Stdin)

	for {
		text, _ := reader.ReadString('\n')
		transactionChan <- c.ChanStruct{Message: text, GenTime: GetTime()}
	}
}

func GetTime() *timestamppb.Timestamp {
	return timestamppb.Now()
}

func ReadConfigFile(configFile string) map[string]string {
	peerMap := make(map[string]string)

	file, err := os.Open(configFile)
	if err != nil {
		fmt.Println("Error reading file")
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	scanner.Scan()

	numNodes, err := strconv.Atoi(scanner.Text())
	if err != nil {
		fmt.Println("Error reading number of nodes")
	}

	for i := 0; i < numNodes; i++ {
		var nodeIdentifier, host, port string
		scanner.Scan()
		fmt.Sscanf(scanner.Text(), "%s %s %s", &nodeIdentifier, &host, &port)
		peerMap[nodeIdentifier] = host + ":" + port

	}

	return peerMap
}
