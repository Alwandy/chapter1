package ratelimit

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"time"
)

type ratelimit interface {
	ReadFile(fileName string) logEntries
	ban()
	unban()
}

type logEntries struct {
	logEntry map[*logEntry] int
}

type logEntry struct {
	Ip float64
	Date time.Time
}

func ReadFile(fileName string) logEntries {
	file, err := os.Open(fileName)
	if err != nil {
		log.Fatalf("[ERROR] %s", err)
	}
	defer file.Close()

	lines := bufio.NewScanner(file)
	for lines.Scan() {
		fmt.Println(lines.Text())
	}
	if err := lines.Err(); err != nil {
		log.Fatalf("[ERROR] %s", err)
	}
	return logEntries{}
}