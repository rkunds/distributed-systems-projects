package main

import (
	"MP0/app/utils"
	"bufio"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

func getTime() float64 {
	return float64(time.Now().UnixNano()) / 1e9
}

// events are printed as below
// [current time] [node_name] [message]
// on client connect, print
// [current time] - [node_name] connected
// on client disconnect, print
// [current time] - [node_name] disconnected

func main() {
	arguments := os.Args

	if len(arguments) != 2 {
		fmt.Println("Please provide a port number")
		return
	}

	SERVER_PORT := arguments[1]

	fmt.Println("Starting logger server on port ", SERVER_PORT)

	ln, err := net.Listen("tcp", ":"+SERVER_PORT)
	if err != nil {
		fmt.Println(err)
		return
	}

	defer ln.Close() // this will be called at the end of the function

	// create a logger
	logger := utils.NewLogger("analysis/data.log")
	data_logger := utils.NewLogger("output.log")
	defer logger.Close()
	defer data_logger.Close()
	for {
		conn, err := ln.Accept()
		if err != nil {
			fmt.Println(err)
			return
		}

		go handleConnection(conn, logger, data_logger)
	}
}

func handleConnection(conn net.Conn, logger *utils.Logger, data_logger *utils.Logger) {
	node_name, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil {
		fmt.Println(err)
		return
	}

	node_name = strings.TrimSpace(node_name)
	connect_time := getTime()

	fmt.Printf("%.9f - %s connected\n", connect_time, node_name)
	reader := bufio.NewReader(conn)
	for {
		message_log, err := reader.ReadString('\n')
		if err != nil {
			fmt.Printf("%.9f - %s disconnected\n", getTime(), node_name)
			return
		}

		message_size := len(message_log)
		words := strings.Fields(message_log) // time is the first word, message is the rest
		recv_time := getTime()
		send_time, err := strconv.ParseFloat(words[0], 64)
		if err != nil {
			fmt.Fprint(os.Stderr, err)

			return
		}

		// print the message
		fmt.Printf("%.9f %s %s\n", recv_time, node_name, words[1])

		// log the message, we use this to calculate latency and bandwith at a later time
		logger.Log(fmt.Sprintf("%.9f,%.9f,%d", getTime(), recv_time-send_time, message_size))
		// data_logger.Log(message_log)
	}
}
