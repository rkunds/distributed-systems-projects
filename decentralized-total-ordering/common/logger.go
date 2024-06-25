package common

import (
	"log"
	"os"
	"sync"
)

type Logger struct {
	ch chan string
	wg sync.WaitGroup
}

func NewLogger(fileName string) *Logger {
	f, err := os.OpenFile(fileName, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatal(err)
	}

	l := &Logger{
		ch: make(chan string),
	}
	l.wg.Add(1)

	go func() {
		defer l.wg.Done()
		for msg := range l.ch {
			f.WriteString(msg + "\n")
		}
		f.Close()
	}()

	return l
}

func (l *Logger) Log(msg string) {
	l.ch <- msg
}

func (l *Logger) Close() {
	close(l.ch)
	l.wg.Wait()
}
