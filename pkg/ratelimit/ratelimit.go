package ratelimit

import (
	"bufio"
	"log"
	"os"
	"github.com/Alwandy/chapter1/pkg/ringbuffer"

)

type logEntries map[int]string
func init() {
	ringbuffer.initialize

}
// StartProcessingLogFile(fileName string) will read the .log file specified when calling the method and build the map object and execute the ratelimiter func
func StartProcessingLogFile(fileName string) {
	file, err := os.Open(fileName)
	if err != nil {
		log.Fatalf("[ERROR] %s", err)
	}
	defer file.Close() // Important to close file when done reading file
	i := 0 // Just ID incrementer
	logentries := logEntries{}
	lines := bufio.NewScanner(file)
	for lines.Scan() {
		logentries[i] = lines.Text()
		i++
	}
	if err := lines.Err(); err != nil {
	log.Fatalf("[ERROR] %s", err)
	}
}