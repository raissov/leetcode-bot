package main

import (
	"log"
	"os"
)

func main() {
	log.Println("LeetCode Bot starting...")

	token := os.Getenv("TOKEN")
	if token == "" {
		log.Fatal("TOKEN environment variable is required")
	}
}
