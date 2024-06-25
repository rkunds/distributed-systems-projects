package common

import (
	"fmt"
	"log"
	"sort"
	"strconv"
	"strings"
	"sync"
)

type TransactionManager struct {
	AccountMap map[string]int
	Log        *Logger
	Lock       sync.Mutex
}

func NewTransactionManager() *TransactionManager {
	t := &TransactionManager{
		AccountMap: make(map[string]int),
		Lock:       sync.Mutex{},
	}

	return t
}

func (t *TransactionManager) AddTransaction(transaction string) {
	fields := strings.Fields(transaction)
	t.Lock.Lock()
	defer t.Lock.Unlock()
	if fields[0] == "DEPOSIT" {
		account := fields[1]

		amount, err := strconv.Atoi(fields[2])
		if err != nil {
			log.Println("Error parsing amount")
			return
		}
		_, ok := t.AccountMap[account]
		if !ok {
			t.AccountMap[account] = 0
		}

		t.AccountMap[account] += amount
	} else if fields[0] == "TRANSFER" {
		from := fields[1]
		to := fields[3]
		amount, err := strconv.Atoi(fields[4])

		if err != nil {
			log.Println("Error parsing amount")
			return
		}

		_, okFrom := t.AccountMap[from]
		_, okTo := t.AccountMap[to]

		if okFrom && okTo {
			if t.AccountMap[from] >= amount {
				t.AccountMap[from] -= amount
				t.AccountMap[to] += amount
			}
		}
	}

	if len(t.AccountMap) == 0 {
		return
	}

	balances := "BALANCES "
	account_list := make([]string, 0, len(t.AccountMap))
	for account := range t.AccountMap {
		account_list = append(account_list, account)
	}

	sort.Strings(account_list)

	for _, account := range account_list {
		if t.AccountMap[account] != 0 {
			balances += account + ":" + strconv.Itoa(t.AccountMap[account]) + " "
		}
	}

	fmt.Println(balances)
}
