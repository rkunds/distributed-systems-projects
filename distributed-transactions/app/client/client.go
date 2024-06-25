package main

import (
	"bufio"
	"context"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"time"

	pb "MP3/proto"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func main() {
	clientID := os.Args[1]
	configFile := os.Args[2]

	config, err := ioutil.ReadFile(configFile)
	if err != nil {
		panic(err)
	}

	servers := make(map[int]string)
	lines := strings.Split(string(config), "\n")
	for i, line := range lines {
		parts := strings.Split(line, " ")
		_, addr, port := parts[0], parts[1], parts[2]
		servers[i] = addr + ":" + port
	}

	reader := bufio.NewReader(os.Stdin)

	for {
		text, _ := reader.ReadString('\n')
		text = strings.TrimSpace(text)
		if text == "BEGIN" {
			StartTransaction(clientID, servers, reader)
		} else {
			continue
		}
	}
}

func StrError(err pb.Error) string {
	switch err {
	case pb.Error_OK:
		return "OK"
	case pb.Error_ABORTED:
		return "ABORTED"
	case pb.Error_NOT_FOUND:
		return "ABORTED, NOT FOUND"
	default:
		return "UNKNOWN ERROR"
	}
}

func StartTransaction(clientID string, servers map[int]string, reader *bufio.Reader) {
	randCoord := rand.Intn(len(servers))
	coordAddr := servers[randCoord]

	var opts []grpc.DialOption
	opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))

	conn, _ := grpc.Dial(coordAddr, opts...)
	client := pb.NewBankClient(conn)

	timestamp := time.Now()
	resp, err := client.CoordSendCommand(
		context.Background(), &pb.Command{
			TxnID:    timestamppb.New(timestamp),
			ClientID: clientID,
			Action:   pb.Action_BEGIN,
		},
	)

	if err != nil {
		panic(err)
	}

	if resp.Error != pb.Error_OK {
		panic(StrError(resp.Error))
	}

	fmt.Println(StrError(resp.Error))
	for {
		text, _ := reader.ReadString('\n')
		text = strings.TrimSpace(text)

		parts := strings.Split(text, " ")
		action := parts[0]

		switch action {
		case "DEPOSIT":
			acctInfo := strings.Split(parts[1], ".")
			branch := acctInfo[0]
			account := acctInfo[1]

			value, _ := strconv.Atoi(parts[2])

			resp, err := client.CoordSendCommand(
				context.Background(), &pb.Command{
					TxnID:     timestamppb.New(timestamp),
					ClientID:  clientID,
					Action:    pb.Action_DEPOSIT,
					BranchID:  branch,
					AccountID: account,
					Amount:    int32(value),
				},
			)

			if err != nil {
				panic(err)
			}

			fmt.Println(StrError(resp.Error))

			if resp.Error == pb.Error_ABORTED || resp.Error == pb.Error_NOT_FOUND {
				client.CoordSendCommand(
					context.Background(), &pb.Command{
						TxnID:    timestamppb.New(timestamp),
						ClientID: clientID,
						Action:   pb.Action_ABORT,
					},
				)

				return
			}

		case "WITHDRAW":
			acctInfo := strings.Split(parts[1], ".")
			branch := acctInfo[0]
			account := acctInfo[1]

			value, _ := strconv.Atoi(parts[2])

			resp, err := client.CoordSendCommand(
				context.Background(), &pb.Command{
					TxnID:     timestamppb.New(timestamp),
					ClientID:  clientID,
					Action:    pb.Action_WITHDRAW,
					BranchID:  branch,
					AccountID: account,
					Amount:    int32(value),
				},
			)

			if err != nil {
				panic(err)
			}

			fmt.Println(StrError(resp.Error))

			if resp.Error == pb.Error_ABORTED || resp.Error == pb.Error_NOT_FOUND {
				client.CoordSendCommand(
					context.Background(), &pb.Command{
						TxnID:    timestamppb.New(timestamp),
						ClientID: clientID,
						Action:   pb.Action_ABORT,
					},
				)

				return
			}
		case "BALANCE":
			acctInfo := strings.Split(parts[1], ".")
			branch := acctInfo[0]
			account := acctInfo[1]

			resp, err := client.CoordSendCommand(
				context.Background(), &pb.Command{
					TxnID:     timestamppb.New(timestamp),
					ClientID:  clientID,
					Action:    pb.Action_BALANCE,
					BranchID:  branch,
					AccountID: account,
				},
			)

			if err != nil {
				panic(err)
			}

			if resp.Error == pb.Error_NOT_FOUND {
				fmt.Println("ABORTED, NOT FOUND")

				client.CoordSendCommand(
					context.Background(), &pb.Command{
						TxnID:    timestamppb.New(timestamp),
						ClientID: clientID,
						Action:   pb.Action_ABORT,
					},
				)

				return
			}

			fmt.Println(parts[1], "=", resp.Value)

		case "COMMIT":
			resp, err := client.CoordSendCommand(
				context.Background(), &pb.Command{
					TxnID:    timestamppb.New(timestamp),
					ClientID: clientID,
					Action:   pb.Action_COMMIT,
				},
			)

			if err != nil {
				panic(err)
			}

			if resp.Error == pb.Error_OK {
				fmt.Println("COMMIT OK")
				return
			}

			fmt.Println("ABORTED")
			client.CoordSendCommand(
				context.Background(), &pb.Command{
					TxnID:    timestamppb.New(timestamp),
					ClientID: clientID,
					Action:   pb.Action_ABORT,
				},
			)

			return
		case "ABORT":
			_, err := client.CoordSendCommand(
				context.Background(), &pb.Command{
					TxnID:    timestamppb.New(timestamp),
					ClientID: clientID,
					Action:   pb.Action_ABORT,
				},
			)

			if err != nil {
				panic(err)
			}

			fmt.Println("ABORTED")
			return
		}

	}
}
