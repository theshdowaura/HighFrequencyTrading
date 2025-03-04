package main

import (
	"log"

	"HighFrequencyTrading/cmd"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	if err := cmd.Execute(); err != nil {
		log.Fatalf("[Error] %v", err)
	}
}
