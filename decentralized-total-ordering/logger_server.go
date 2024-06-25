package main

import (
	c "CS425MP1/common"
	pb "CS425MP1/proto"
	"context"
	"fmt"
	"net"
	"os"

	"google.golang.org/grpc"
)

func main() {
	args := os.Args

	if len(args) != 2 {
		fmt.Println("Usage: ./logger <port>")
		return
	}

	port := args[1]

	lis, err := net.Listen("tcp", ":"+port)
	if err != nil {
		panic(err)
	}

	service := grpc.NewServer()
	pb.RegisterMessageServiceServer(service, &LoggingServer{
		Logger: c.NewLogger("logs/Transaction.log"),
	})
	if err := service.Serve(lis); err != nil {
		panic(err)
	}
}

type LoggingServer struct {
	pb.UnimplementedMessageServiceServer
	Logger *c.Logger
}

func (s *LoggingServer) Log(ctx context.Context, in *pb.LogMessage) (*pb.Empty, error) {
	log_message := ""
	time := in.Time
	float_time := float64(time.GetSeconds()) + float64(time.GetNanos())/float64(1e9)

	log_message = fmt.Sprintf("%d,%t,%s,%.9f", in.TransactionID, in.Generated, in.SenderNode, float_time)
	fmt.Println(log_message)
	s.Logger.Log(log_message)
	return &pb.Empty{}, nil
}
