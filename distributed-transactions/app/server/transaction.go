package main

import (
	"sync"
	"time"
)

type status int
type action int

const (
	STARTED status = iota
	PREPARED
	COMMITTED
	ABORTED
)

const (
	DEPOSIT action = iota
	WITHDRAW
	BALANCE
	BEGIN
	ABORT
	COMMIT
)

type Command struct {
	CommandType action
	AccountID   string
	Amount      int
}

type Transaction struct {
	TransactionID time.Time
	ClientID      string
	CoordinatorID string

	AccountsCreated sync.Map // treat as set, key is accountID, value is bool 
	AccountsChanged sync.Map // treat as set, key is accountID

	Responses sync.Map // map of responses that we have recieved

	Commands  []Command
	TxnStatus status

	// rw lock
	Lock *sync.RWMutex
}

func InitTxn(clientID, CoordinatorID string, txnID time.Time) *Transaction {
	return &Transaction{
		TransactionID:   txnID,
		ClientID:        clientID,
		CoordinatorID:   CoordinatorID,
		AccountsCreated: sync.Map{},
		Commands:        []Command{},
		TxnStatus:       STARTED,
		Lock:            &sync.RWMutex{},
	}
}

func (txn *Transaction) AddCommand(command Command) {
	txn.Lock.Lock()
	defer txn.Lock.Unlock()
	txn.Commands = append(txn.Commands, command)
}

func (txn *Transaction) AddAccount(accountID string) {
	txn.Lock.Lock()
	defer txn.Lock.Unlock()
	txn.AccountsCreated.Store(accountID, true)
}
