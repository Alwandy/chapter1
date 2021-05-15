package main

import (
	"github.com/Alwandy/chapter1/pkg/ratelimit"
	"log"
)

func main() {
	log.Println("[INFO] Starting rate limiter application")
	ratelimit.StartProcessingLogFile("test/sample.log")
}