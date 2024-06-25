package main

import (
	"sort"
	"sync"
	"time"
)

func gt(t1, t2 time.Time) bool {
	// t1 > t2
	return t1.After(t2)
}

func lt(t1, t2 time.Time) bool {
	return t1.Before(t2)
}

func eq(t1, t2 time.Time) bool {
	return t1.Equal(t2)
}

func ge(t1, t2 time.Time) bool {
	return gt(t1, t2) || eq(t1, t2)
}

func le(t1, t2 time.Time) bool {
	return lt(t1, t2) || eq(t1, t2)
}

type TentativeWrite struct {
	TransactionID time.Time
	Value         int
	IsCommitted   bool
}

// struct for object
type Account struct {
	Lock  sync.Mutex // used when changing account fields
	Notif *sync.Cond // used to notify a given operation that a value has been committed or aborted, and then we can read the value once again

	Exists         bool // set true once we have successfully commited a value
	CommittedValue int
	TransactionID  time.Time

	ReadTimestamps []time.Time
	PendingWrites  []TentativeWrite
}

// sort times for pending writes
func (a *Account) SortTimes() {
	sort.Slice(a.ReadTimestamps, func(i, j int) bool {
		return a.ReadTimestamps[i].Before(a.ReadTimestamps[j])
	})

	sort.Slice(a.PendingWrites, func(i, j int) bool {
		return a.PendingWrites[i].TransactionID.Before(a.PendingWrites[j].TransactionID)
	})
}

func (a *Account) SortReads() {
	sort.Slice(a.ReadTimestamps, func(i, j int) bool {
		return a.ReadTimestamps[i].Before(a.ReadTimestamps[j])
	})
}

func (a *Account) SortWrites() {
	sort.Slice(a.PendingWrites, func(i, j int) bool {
		return a.PendingWrites[i].TransactionID.Before(a.PendingWrites[j].TransactionID)
	})
}

type Branch struct {
	BranchName  string
	AccountsMap map[string]*Account // map of accounts
	AccountRW   *sync.RWMutex
}

// create a new Branch
func NewBranch(branchName string) *Branch {
	return &Branch{
		BranchName:  branchName,
		AccountsMap: make(map[string]*Account),
		AccountRW:   &sync.RWMutex{},
	}
}

func (b *Branch) CreateAccount(accountID string) {
	b.AccountRW.Lock()
	defer b.AccountRW.Unlock()

	b.AccountsMap[accountID] = NewAccount(accountID)
}

func (b *Branch) GetAccount(accountID string) *Account {
	b.AccountRW.RLock()
	defer b.AccountRW.RUnlock()

	return b.AccountsMap[accountID]
}

func (b *Branch) ContainsAccount(accountID string) bool {
	b.AccountRW.RLock()
	defer b.AccountRW.RUnlock()

	_, ok := b.AccountsMap[accountID]
	return ok
}

func NewAccount(accountID string) *Account {
	a := &Account{
		Lock: sync.Mutex{},
		// use Account.Lock for notif
		Exists:         false,
		CommittedValue: 0,
		TransactionID:  time.Time{},
		ReadTimestamps: []time.Time{
			{},
		}, // init with zero timestamp
		PendingWrites: []TentativeWrite{},
	}

	a.Notif = sync.NewCond(&a.Lock)
	return a
}

type W_STATUS int

const (
	W_OK = iota
	W_ABORT
)

func (a *Account) ReqWrite(txnID time.Time, value int) W_STATUS {
	a.Lock.Lock()
	defer a.Lock.Unlock()

	lastWrite := a.TransactionID
	lastRead := a.ReadTimestamps[len(a.ReadTimestamps)-1]

	if ge(txnID, lastRead) && gt(txnID, lastWrite) {
		// perform tentative write
		// if txnID is already in pending writes, update value
		for i, write := range a.PendingWrites {
			if eq(write.TransactionID, txnID) {
				a.PendingWrites[i].Value = value
				return W_OK
			}
		}

		// add new write
		a.PendingWrites = append(a.PendingWrites, TentativeWrite{
			TransactionID: txnID,
			Value:         value,
			IsCommitted:   false,
		})

		a.SortWrites()

		return W_OK
	}

	return W_ABORT
}

type R_STATUS int

const (
	R_OK = iota
	R_NOT_FOUND
	R_ABORT
)

func (a *Account) ReqRead(txnID time.Time) (int, R_STATUS) {
	a.Lock.Lock()

	lastWrite := a.TransactionID
	if gt(txnID, lastWrite) {
		wTxn, wVal := a.TransactionID, a.CommittedValue
		wCom := a.Exists

		for i := 0; i < len(a.PendingWrites); i++ {
			if ge(txnID, a.PendingWrites[i].TransactionID) {
				wTxn = a.PendingWrites[i].TransactionID
				wVal = a.PendingWrites[i].Value
				wCom = a.PendingWrites[i].IsCommitted
			} else {
				break
			}
		}

		if eq(wTxn, time.Time{}) {
			a.Lock.Unlock()
			return 0, R_NOT_FOUND
		}

		if wCom {
			a.ReadTimestamps = append(a.ReadTimestamps, txnID)
			a.SortReads()
			a.SortWrites()
			a.Lock.Unlock()
			return wVal, R_OK
		}

		if eq(wTxn, txnID) {
			a.Lock.Unlock()
			return wVal, R_OK
		}

		a.Notif.Wait()
		a.Lock.Unlock()
		return a.ReqRead(txnID)
	}

	a.Lock.Unlock()
	return 0, R_ABORT
}

type C_STATUS int

const (
	C_OK    = iota
	C_ABORT // if txnID is not the earliest in pending writes
)

func (a *Account) CanCommit(txnID time.Time) bool {
	a.Lock.Lock()
	defer a.Lock.Unlock()

	for _, ts := range a.PendingWrites {
		if eq(ts.TransactionID, txnID) {
			return ts.Value > 0
		}
	}

	return true
}

func (a *Account) Commit(txnID time.Time) C_STATUS {
	a.Lock.Lock()
	// defer a.Lock.Unlock()

	if len(a.PendingWrites) == 0 {
		a.Lock.Unlock()
		return C_OK // nothing to commit for this account, everything has been committed
	}

	if eq(a.PendingWrites[0].TransactionID, txnID) {
		if a.PendingWrites[0].Value < 0 {
			a.Lock.Unlock()
			return C_ABORT
		}
		a.CommittedValue = a.PendingWrites[0].Value
		a.TransactionID = txnID
		a.Exists = true

		a.PendingWrites = a.PendingWrites[1:]
		a.SortWrites() // no need to sort...

		a.Notif.Broadcast()
		a.Lock.Unlock()
		return C_OK
	} else {
		a.Notif.Wait()
		a.Lock.Unlock()
		return a.Commit(txnID)
	}
}

func (a *Account) Abort(txnID time.Time) {
	a.Lock.Lock()
	defer a.Lock.Unlock()

	if len(a.PendingWrites) == 0 {
		return
	}

	txnIdx := -1
	for i, write := range a.PendingWrites {
		if eq(write.TransactionID, txnID) {
			txnIdx = i
			break
		}
	}

	// remove txnID from read timestamps
	for i, ts := range a.ReadTimestamps {
		if eq(ts, txnID) {
			a.ReadTimestamps = append(a.ReadTimestamps[:i], a.ReadTimestamps[i+1:]...)
			break
		}
	}

	if txnIdx == -1 {
		return
	}

	a.Notif.Broadcast()

	a.PendingWrites = append(a.PendingWrites[:txnIdx], a.PendingWrites[txnIdx+1:]...)
	a.SortWrites()
	a.SortReads()

}
