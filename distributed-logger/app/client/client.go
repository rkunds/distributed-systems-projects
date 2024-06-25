package main

import (
	"bufio"
	"fmt"
	"net"
	"os"
)

func main() {
	arguments := os.Args

	// start client node by node_name, addr, port
	if len(arguments) != 4 {
		fmt.Println("Please provide a node_name, addr, port")
		return
	}

	node_name := arguments[1]
	addr := arguments[2]
	port := arguments[3]

	CONNECT := addr + ":" + port

	c, err := net.Dial("tcp", CONNECT)
	if err != nil {
		fmt.Println(err)
		return
	}

	defer c.Close() // this will be called at the end of the function

	// the client will send the node_name to the server
	fmt.Fprintf(c, node_name+"\n")

	count := 0
	reader := bufio.NewReader(os.Stdin)

	for {
		// read in input from stdin
		count += 1

		text, err := reader.ReadString('\n')
		if err != nil {
			fmt.Println(err)
			fmt.Println("count: ", count)
			return
		}
		fmt.Fprint(c, text)
	}

}
