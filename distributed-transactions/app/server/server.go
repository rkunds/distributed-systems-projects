package main

import (
	pb "MP3/proto"
	"context"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Bank struct {
	pb.UnimplementedBankServer

	BankID string

	PeersAddrs map[string]string
	Peers      map[string]*grpc.ClientConn

	Branches *Branch

	CoordinatorTxns *CMap
	BranchTxns      *CMap
}

func NewBank(bankID string, peersAddrs map[string]string) *Bank {
	return &Bank{
		BankID:     bankID,
		PeersAddrs: peersAddrs,
		Peers:      make(map[string]*grpc.ClientConn),

		Branches: NewBranch(bankID),

		CoordinatorTxns: NewCMap(),
		BranchTxns:      NewCMap(),
	}
}

func (b *Bank) StartService() {
	serverAddr := b.PeersAddrs[b.BankID]

	lis, err := net.Listen("tcp", serverAddr)
	if err != nil {
		panic(err)
	}

	service := grpc.NewServer()
	pb.RegisterBankServer(service, b)

	wg := sync.WaitGroup{}
	wg.Add(1)

	go func() {
		defer wg.Done()
		if err := service.Serve(lis); err != nil {
			panic(err)
		}

	}()

	b.InitPeers()

	wg.Wait()
}

func (b *Bank) InitPeers() {
	var opts []grpc.DialOption
	opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))

	for nodeID, addr := range b.PeersAddrs {
		for {
			conn, _ := grpc.Dial(addr, opts...)
			client := pb.NewBankClient(conn)

			_, m_err := client.Ping(context.Background(), &pb.Empty{})

			if m_err == nil {
				b.Peers[nodeID] = conn
				fmt.Fprintln(os.Stderr, "Connected to", nodeID)
				break
			} else {
				time.Sleep(1 * time.Second)
			}
		}
	}

	fmt.Fprintln(os.Stderr, "All peers connected")
}

func (b *Bank) PrintBalances() {
	b.Branches.AccountRW.RLock()
	defer b.Branches.AccountRW.RUnlock()

	for accountID, account := range b.Branches.AccountsMap {
		if account.CommittedValue > 0 {
			fmt.Print(accountID, " ", account.CommittedValue, ";")
		}
	}
	fmt.Print("\n")
}

func main() {
	bankID := os.Args[1]
	peersAddrs := make(map[string]string)

	configFile := os.Args[2]
	config, err := ioutil.ReadFile(configFile)
	if err != nil {
		panic(err)
	}

	lines := strings.Split(string(config), "\n")
	for _, line := range lines {
		parts := strings.Split(line, " ")
		id, addr, port := parts[0], parts[1], parts[2]

		peersAddrs[id] = addr + ":" + port
	}

	bank := NewBank(bankID, peersAddrs)
	bank.StartService()
}
