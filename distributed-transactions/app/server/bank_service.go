package main

import (
	pb "MP3/proto"
	"context"

	"google.golang.org/grpc"
)

func ConvertAction(action pb.Action) action {
	switch action {
	case pb.Action_DEPOSIT:
		return DEPOSIT
	case pb.Action_WITHDRAW:
		return WITHDRAW
	case pb.Action_ABORT:
		return ABORT
	case pb.Action_COMMIT:
		return COMMIT
	case pb.Action_BEGIN:
		return BEGIN
	case pb.Action_BALANCE:
		return BALANCE
	default:
		return -1
	}
}

func (b *Bank) Ping(ctx context.Context, in *pb.Empty) (*pb.Empty, error) {
	return &pb.Empty{}, nil
}

func (b *Bank) CoordSendCommand(ctx context.Context, in *pb.Command) (*pb.Response, error) {
	// handles the command from the client
	txnID := in.TxnID.AsTime()
	clientID := in.ClientID

	coordinatorID := b.BankID
	action := ConvertAction(in.Action)
	branchID := in.BranchID
	accountID := in.AccountID
	amount := in.Amount

	// see if txn exists in coordinatortxns
	txn, ok := b.CoordinatorTxns.Load(txnID)
	if !ok {
		// create new txn
		txn = InitTxn(clientID, coordinatorID, txnID)
		b.CoordinatorTxns.Store(txnID, txn)
	}

	tx := txn.(*Transaction)

	cmd := Command{
		CommandType: action,
		AccountID:   accountID,
		Amount:      int(amount),
	}

	tx.AddCommand(cmd)

	switch action {
	case DEPOSIT:
		// invoke ServerSendCommand RPC
		conn := b.Peers[branchID]
		client := pb.NewBankClient(conn)

		resp, err := client.ServerSendCommand(context.Background(), in)
		if err != nil {
			panic(err)
		}

		return resp, nil

	case WITHDRAW:
		// invoke ServerSendCommand RPC
		conn := b.Peers[branchID]
		client := pb.NewBankClient(conn)

		resp, err := client.ServerSendCommand(context.Background(), in)
		if err != nil {
			panic(err)
		}

		return resp, nil

	case BALANCE:
		// invoke ServerSendCommand RPC
		conn := b.Peers[branchID]
		client := pb.NewBankClient(conn)

		resp, err := client.ServerSendCommand(context.Background(), in)
		if err != nil {
			panic(err)
		}

		return resp, nil

	case BEGIN:
		return &pb.Response{
			TxnId:    in.TxnID,
			ServerID: b.BankID,
			Error:    pb.Error_OK,
		}, nil

	case ABORT:
		// invoke SENDABORT RPC to all branches with the txnID
		tx.Lock.Lock()
		defer tx.Lock.Unlock()

		tx.TxnStatus = ABORTED

		// send abort to all branches
		for branchID := range b.Peers {
			conn := b.Peers[branchID]
			client := pb.NewBankClient(conn)
			_, err := client.SendAbort(context.Background(), &pb.Transaction{TxnId: in.TxnID})
			if err != nil {
				panic(err)
			}
		}

		return &pb.Response{
			TxnId:    in.TxnID,
			ServerID: b.BankID,
			Error:    pb.Error_OK,
		}, nil

	case COMMIT:
		// invoke SENDCOMMIT RPC to all branches with the txnID
		tx.Lock.Lock()
		defer tx.Lock.Unlock()

		tx.TxnStatus = PREPARED
		count := make(chan int, len(b.Peers))

		// ask prepare to all branches
		for branchID := range b.Peers {
			conn := b.Peers[branchID]
			go func(conn *grpc.ClientConn, in *pb.Transaction) {
				client := pb.NewBankClient(conn)
				resp, err := client.SendPrepare(context.Background(), in)
				if err != nil {
					panic(err)
				}

				if resp.Error == pb.Error_OK {
					count <- 1
				} else {
					count <- 0
				}
			}(conn, &pb.Transaction{TxnId: in.TxnID})
		}

		// wait for all branches to respond
		prepareCount := 0
		for i := 0; i < len(b.Peers); i++ {
			prepareCount += <-count
		}

		if prepareCount == len(b.Peers) {
			// send commit to all branches
			for branchID := range b.Peers {
				conn := b.Peers[branchID]
				client := pb.NewBankClient(conn)
				_, err := client.SendCommit(context.Background(), &pb.Transaction{TxnId: in.TxnID})
				if err != nil {
					panic(err)
				}
			}

			return &pb.Response{
				TxnId:    in.TxnID,
				ServerID: b.BankID,
				Error:    pb.Error_OK,
			}, nil
		} else {
			// send abort to all branches
			for branchID := range b.Peers {
				conn := b.Peers[branchID]
				client := pb.NewBankClient(conn)
				_, err := client.SendAbort(context.Background(), &pb.Transaction{TxnId: in.TxnID})
				if err != nil {
					panic(err)
				}
			}

			return &pb.Response{
				TxnId:    in.TxnID,
				ServerID: b.BankID,
				Error:    pb.Error_ABORTED,
			}, nil

		}

	}

	return &pb.Response{}, nil
}

func (b *Bank) ServerSendCommand(ctx context.Context, in *pb.Command) (*pb.Response, error) {
	txnID := in.TxnID.AsTime()
	clientID := in.ClientID

	coordinatorID := b.BankID
	action := ConvertAction(in.Action)
	accountID := in.AccountID
	amount := in.Amount

	_, ok := b.BranchTxns.Load(txnID)
	if !ok {
		t := InitTxn(clientID, coordinatorID, txnID)
		b.BranchTxns.Store(txnID, &t)
	}

	t, ok := b.BranchTxns.Load(txnID)
	if !ok {
		panic("txn not found")
	}

	txn := t.(*Transaction)

	cmd := Command{
		CommandType: action,
		AccountID:   accountID,
		Amount:      int(amount),
	}

	txn.AddCommand(cmd)
	switch action {
	case DEPOSIT:
		if !b.Branches.ContainsAccount(accountID) {
			b.Branches.CreateAccount(accountID)
			txn.AccountsCreated.Store(accountID, true)
		}

		txn.AccountsChanged.Store(accountID, true)
		account := b.Branches.GetAccount(accountID)

		bal, err := account.ReqRead(txnID)
		if err == R_NOT_FOUND {
			account.ReqWrite(txnID, 0)
		}

		if err == R_ABORT {
			return &pb.Response{
				TxnId:    in.TxnID,
				ServerID: b.BankID,
				Error:    pb.Error_ABORTED,
			}, nil
		}

		w_err := account.ReqWrite(txnID, bal+int(amount))
		if w_err == W_ABORT {
			return &pb.Response{
				TxnId:    in.TxnID,
				ServerID: b.BankID,
				Error:    pb.Error_ABORTED,
			}, nil
		}

		return &pb.Response{
			TxnId:    in.TxnID,
			ServerID: b.BankID,
			Error:    pb.Error_OK,
		}, nil

	case BALANCE:
		if !b.Branches.ContainsAccount(accountID) {
			return &pb.Response{
				TxnId:    in.TxnID,
				ServerID: b.BankID,
				Error:    pb.Error_NOT_FOUND,
			}, nil
		}

		account := b.Branches.GetAccount(accountID)
		bal, err := account.ReqRead(txnID)
		if err == R_NOT_FOUND {
			return &pb.Response{
				TxnId:    in.TxnID,
				ServerID: b.BankID,
				Error:    pb.Error_NOT_FOUND,
			}, nil
		}

		if err == R_ABORT {
			return &pb.Response{
				TxnId:    in.TxnID,
				ServerID: b.BankID,
				Error:    pb.Error_ABORTED,
			}, nil
		}

		return &pb.Response{
			TxnId:    in.TxnID,
			ServerID: b.BankID,
			Error:    pb.Error_OK,
			Value:    int32(bal),
		}, nil

	case WITHDRAW:
		if !b.Branches.ContainsAccount(accountID) {
			return &pb.Response{
				TxnId:    in.TxnID,
				ServerID: b.BankID,
				Error:    pb.Error_NOT_FOUND,
			}, nil
		}
		txn.AccountsChanged.Store(accountID, true)

		account := b.Branches.GetAccount(accountID)
		bal, err := account.ReqRead(txnID)
		if err == R_NOT_FOUND {
			return &pb.Response{
				TxnId:    in.TxnID,
				ServerID: b.BankID,
				Error:    pb.Error_NOT_FOUND,
			}, nil
		}

		if err == R_ABORT {
			return &pb.Response{
				TxnId:    in.TxnID,
				ServerID: b.BankID,
				Error:    pb.Error_ABORTED,
			}, nil
		}

		w_err := account.ReqWrite(txnID, bal-int(amount))
		if w_err == W_ABORT {
			return &pb.Response{
				TxnId:    in.TxnID,
				ServerID: b.BankID,
				Error:    pb.Error_ABORTED,
			}, nil
		}

		return &pb.Response{
			TxnId:    in.TxnID,
			ServerID: b.BankID,
			Error:    pb.Error_OK,
		}, nil

	default:
		return &pb.Response{}, nil
	}

}

func (b *Bank) SendPrepare(ctx context.Context, in *pb.Transaction) (*pb.Exception, error) {
	txnID := in.TxnId.AsTime()
	t, ok := b.BranchTxns.Load(txnID)
	if !ok {
		// txn not found, no need to prepare
		return &pb.Exception{
			TxnId: in.TxnId,
			Error: pb.Error_OK,
		}, nil
	} else {
		txn := t.(*Transaction)
		txn.Lock.Lock()
		defer txn.Lock.Unlock()

		if txn.TxnStatus == ABORTED {
			return &pb.Exception{
				TxnId: in.TxnId,
				Error: pb.Error_ABORTED,
			}, nil
		}

		txn.TxnStatus = PREPARED

		// check if all accounts can commit
		txn.AccountsChanged.Range(func(key, value interface{}) bool {
			accountID := key.(string)
			account := b.Branches.GetAccount(accountID)
			if account.CanCommit(txnID) {
				return true
			} else {
				txn.TxnStatus = ABORTED
				return false
			}
		})

		if txn.TxnStatus == ABORTED {
			return &pb.Exception{
				TxnId: in.TxnId,
				Error: pb.Error_ABORTED,
			}, nil
		}

		return &pb.Exception{
			TxnId: in.TxnId,
			Error: pb.Error_OK,
		}, nil
	}
}

func (b *Bank) SendCommit(ctx context.Context, in *pb.Transaction) (*pb.Exception, error) {
	txnID := in.TxnId.AsTime()
	t, ok := b.BranchTxns.Load(txnID)
	if !ok {
		// txn not found, no need to commit
		return &pb.Exception{
			TxnId: in.TxnId,
			Error: pb.Error_OK,
		}, nil
	} else {
		txn := t.(*Transaction)
		txn.Lock.Lock()
		defer txn.Lock.Unlock()

		if txn.TxnStatus == ABORTED {
			return &pb.Exception{
				TxnId: in.TxnId,
				Error: pb.Error_ABORTED,
			}, nil
		}

		txn.TxnStatus = COMMITTED

		// commit all accounts
		txn.AccountsChanged.Range(func(key, value interface{}) bool {
			accountID := key.(string)
			account := b.Branches.GetAccount(accountID)
			account.Commit(txnID)
			return true
		})

		go b.PrintBalances()

		return &pb.Exception{
			TxnId: in.TxnId,
			Error: pb.Error_OK,
		}, nil
	}
}

func (b *Bank) SendAbort(ctx context.Context, in *pb.Transaction) (*pb.Exception, error) {
	txnID := in.TxnId.AsTime()
	t, ok := b.BranchTxns.Load(txnID)
	if !ok {
		// txn not found, no need to abort
		return &pb.Exception{
			TxnId: in.TxnId,
			Error: pb.Error_OK,
		}, nil
	} else {
		txn := t.(*Transaction)
		txn.Lock.Lock()
		defer txn.Lock.Unlock()

		txn.TxnStatus = ABORTED

		// abort all accounts
		txn.AccountsChanged.Range(func(key, value interface{}) bool {
			accountID := key.(string)
			account := b.Branches.GetAccount(accountID)
			account.Abort(txnID)
			return true
		})

		return &pb.Exception{
			TxnId: in.TxnId,
			Error: pb.Error_OK,
		}, nil
	}
}
