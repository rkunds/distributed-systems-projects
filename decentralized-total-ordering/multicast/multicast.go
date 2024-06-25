package multicast

import (
	c "CS425MP1/common"
	"container/heap"
	"context"
	"log"
	"math/rand"
	"net"
	"sync"
	"time"

	pb "CS425MP1/proto"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type Multicast struct {
	pb.UnimplementedMessageServiceServer

	ClientAddrs     map[string]string
	ClientGroup     map[string]*pb.MessageServiceClient
	LogClient       *pb.MessageServiceClient
	DeadNodes       sync.Mutex
	ClientGroupLock sync.Mutex
	NodeID          string

	DeadNodeNotifier chan string

	AgreedSequence   uint64
	ProposedSequence uint64
	SequenceLock     sync.Mutex

	PriorityQueue *c.PriorityQueue
	MessageMap    map[uint64]*c.DataMessage
	QueueLock     sync.Mutex
	DeliveryLock  sync.Mutex
	BroadcastLock sync.Mutex

	TransactionChan chan c.ChanStruct

	TransactionManager *c.TransactionManager

	Logger *c.Logger
}

func NewMulticast(nodeID string, clientAddrs map[string]string, transactionChannel chan c.ChanStruct) *Multicast {
	m := &Multicast{
		ClientAddrs:      clientAddrs,
		ClientGroup:      make(map[string]*pb.MessageServiceClient),
		DeadNodes:        sync.Mutex{}, // add nodes to this list when when they die
		ClientGroupLock:  sync.Mutex{},
		NodeID:           nodeID,
		DeadNodeNotifier: make(chan string),

		AgreedSequence:   0,
		ProposedSequence: 0,
		SequenceLock:     sync.Mutex{},

		PriorityQueue: c.NewPriorityQueue(),
		MessageMap:    make(map[uint64]*c.DataMessage),
		QueueLock:     sync.Mutex{},
		DeliveryLock:  sync.Mutex{},

		BroadcastLock: sync.Mutex{},

		TransactionChan:    transactionChannel,
		TransactionManager: c.NewTransactionManager(),
		LogClient:          nil,

		Logger: c.NewLogger("logs/" + nodeID + ".log"),
	}

	return m
}

func GenerateID() uint64 {
	return rand.Uint64()
}

func (m *Multicast) SetDebugLogger(debugLog *pb.MessageServiceClient) {
	m.LogClient = debugLog
}

func (m *Multicast) Start() {
	serverAddr := m.ClientAddrs[m.NodeID]

	lis, err := net.Listen("tcp", serverAddr)
	if err != nil {
		panic(err)
	}

	service := grpc.NewServer()
	pb.RegisterMessageServiceServer(service, m)

	wg := sync.WaitGroup{}
	wg.Add(1)

	go func() {
		defer wg.Done()
		if err := service.Serve(lis); err != nil {
			panic(err)
		}
	}()

	m.InitializeClients()

	time.Sleep(1 * time.Second)

	go m.NodeDetector()
	go m.TransactionHandler()

	wg.Wait()
}

func (m *Multicast) InitializeClients() {
	var opts []grpc.DialOption
	opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))

	for nodeID, addr := range m.ClientAddrs {
		if nodeID == m.NodeID {
			continue
		}

		for {
			conn, _ := grpc.Dial(addr, opts...)
			client := pb.NewMessageServiceClient(conn)

			_, m_err := client.Init(context.Background(), &pb.Empty{})

			if m_err == nil {
				m.ClientGroupLock.Lock()
				m.ClientGroup[nodeID] = &client
				m.ClientGroupLock.Unlock()
				break
			}

			time.Sleep(1 * time.Second)
		}
	}
}

func (m *Multicast) NodeDetector() {
	for {
		deadNode := <-m.DeadNodeNotifier

		m.ClientGroupLock.Lock()
		delete(m.ClientGroup, deadNode)
		m.ClientGroupLock.Unlock()
	}
}

func (m *Multicast) InitiateTransaction(transaction string, timeGenerated *timestamppb.Timestamp) {
	transactionID := GenerateID()
	m.BroadcastLock.Lock()
	defer m.BroadcastLock.Unlock()

	in := &pb.DataMessage{
		Transaction:   transaction,
		TransactionID: transactionID,
		Sender:        m.NodeID,
	}

	wg := sync.WaitGroup{} 

	seqChannel := make(chan struct {
		uint64
		string
	}, 10)

	for nodeID, client := range m.ClientGroup {
		wg.Add(1)

		if nodeID == m.NodeID {
			continue
		}

		go func(client *pb.MessageServiceClient, recvNode string) {
			resp, err := (*client).SendMessage(context.Background(), in)
			if err != nil {
				m.DeadNodeNotifier <- recvNode
			} else {
				seqChannel <- struct {
					uint64
					string
				}{resp.ProposedSeq, recvNode}
			}

			wg.Done()
		}(client, nodeID)
	}

	wg.Wait()
	close(seqChannel)

	maxProposal := uint64(0)
	proposedNode := ""

	for val := range seqChannel {
		if val.uint64 > maxProposal {
			maxProposal = val.uint64
			proposedNode = val.string
		}
	}

	m.SequenceLock.Lock()
	m.AgreedSequence = max(m.AgreedSequence, maxProposal)
	m.SequenceLock.Unlock()

	m.QueueLock.Lock()

	dataMessage := &c.DataMessage{
		Transaction:   transaction,
		TransactionID: transactionID,
		SenderNode:    m.NodeID,
		Priority:      maxProposal,
		PropserNode:   proposedNode,
		Deliverable:   true,
	}

	heap.Push(m.PriorityQueue, dataMessage)
	m.MessageMap[transactionID] = dataMessage
	heap.Init(m.PriorityQueue)

	m.QueueLock.Unlock()

	finalAck := &pb.FinalAck{
		TransactionID: transactionID,
		FinalSeq:      maxProposal,
		PropsedNode:   proposedNode,
	}

	finalWg := sync.WaitGroup{}

	go func() {
		if m.LogClient != nil {
			lm := &pb.LogMessage{
				TransactionID: transactionID,
				Generated:     true,
				Time:          timeGenerated,
				SenderNode:    m.NodeID,
			}

			(*m.LogClient).Log(context.Background(), lm)
		}
	}()

	go m.DeliverMessages()

	m.ClientGroupLock.Lock()
	for nodeID, client := range m.ClientGroup {
		if nodeID == m.NodeID {
			continue
		}

		finalWg.Add(1)
		go func(client *pb.MessageServiceClient, nodeID string) {
			_, err := (*client).SendFinal(context.Background(), finalAck)
			if err != nil {
				m.DeadNodeNotifier <- nodeID
			}

			finalWg.Done()
		}(client, nodeID)
	}
	m.ClientGroupLock.Unlock()

	finalWg.Wait()
}

func (m *Multicast) TransactionHandler() {
	for {
		transaction := <-m.TransactionChan
		message := transaction.Message
		genTime := transaction.GenTime
		go m.InitiateTransaction(message, genTime)
	}
}

func (m *Multicast) Init(ctx context.Context, in *pb.Empty) (*pb.Empty, error) {
	return &pb.Empty{}, nil
}

func (m *Multicast) SendMessage(ctx context.Context, in *pb.DataMessage) (*pb.Ack, error) {
	m.SequenceLock.Lock()

	// P = max(A, P) + 1
	m.ProposedSequence = max(m.ProposedSequence, m.AgreedSequence) + 1
	propsal := m.ProposedSequence
	m.SequenceLock.Unlock()


	// add the message to the priority queue
	m.QueueLock.Lock()
	dataMessage := &c.DataMessage{
		Transaction:   in.Transaction,
		TransactionID: in.TransactionID,
		SenderNode:    in.Sender,
		Priority:      propsal,
		PropserNode:   m.NodeID,
		Deliverable:   false,
	}


	heap.Push(m.PriorityQueue, dataMessage)
	m.MessageMap[in.TransactionID] = dataMessage
	m.QueueLock.Unlock()

	// send the ack
	return &pb.Ack{TransactionID: in.TransactionID,
		ProposedSeq: propsal,
		Sender:      m.NodeID}, nil
}

func (m *Multicast) SendFinal(ctx context.Context, in *pb.FinalAck) (*pb.Empty, error) {
	m.QueueLock.Lock()
	dataMessage, err := m.MessageMap[in.TransactionID]

	if !err {
		m.QueueLock.Unlock()
		log.Println("Transaction not found, how did we get here ?!")
		return &pb.Empty{}, nil
	}

	isDeliverable := dataMessage.Deliverable

	if isDeliverable {
		// already seen this message, no need to rebroadcast
		m.QueueLock.Unlock()
		return &pb.Empty{}, nil
	}

	m.SequenceLock.Lock()
	m.AgreedSequence = max(m.AgreedSequence, in.FinalSeq)
	m.SequenceLock.Unlock()

	dataMessage.Priority = in.FinalSeq
	dataMessage.PropserNode = in.PropsedNode
	dataMessage.Deliverable = true

	heap.Init(m.PriorityQueue)

	m.QueueLock.Unlock()

	go m.BMulticast(in)
	go m.DeliverMessages()

	return &pb.Empty{}, nil
}

func (m *Multicast) DeliverMessages() {
	m.DeliveryLock.Lock()
	defer m.DeliveryLock.Unlock()
	m.QueueLock.Lock()
	defer m.QueueLock.Unlock()
	heap.Init(m.PriorityQueue)

	// iterate through pqueue and deliver all the messages that are deliverable
	for {
		msg, ok := m.PriorityQueue.Peek()
		if !ok {
			break
		}

		if msg.(*c.DataMessage).Deliverable {
			transactionString := msg.(*c.DataMessage).Transaction
			go func() {
				if m.LogClient != nil {
					lm := &pb.LogMessage{
						TransactionID: msg.(*c.DataMessage).TransactionID,
						Generated:     false,
						Time:          timestamppb.Now(),
						SenderNode:    m.NodeID,
					}

					(*m.LogClient).Log(context.Background(), lm)
				}
			}()

			m.Logger.Log(transactionString)

			m.TransactionManager.AddTransaction(transactionString)

			heap.Pop(m.PriorityQueue)
		} else {
			break
		}
	}

}

func (m *Multicast) BMulticast(in *pb.FinalAck) {
	m.BroadcastLock.Lock()
	defer m.BroadcastLock.Unlock()
	in.Sender = m.NodeID

	wg := sync.WaitGroup{} // run sends in parallel

	for nodeID, client := range m.ClientGroup {
		wg.Add(1)
		if nodeID == m.NodeID {
			continue
		}

		go func(client *pb.MessageServiceClient, nodeID string) {
			_, err := (*client).SendFinal(context.Background(), in)
			if err != nil {
				m.DeadNodeNotifier <- nodeID
			}

			wg.Done()
		}(client, nodeID)
	}

	wg.Wait()
}
