package main

import (
	"log"
	"os"
)

func main() {
	logger := log.New(os.Stdout, "subtitle-review ", log.LstdFlags|log.LUTC)
	cfg, err := parseConfig()
	if err != nil {
		logger.Fatal(err)
	}
	if err := run(cfg, logger); err != nil {
		logger.Fatal(err)
	}
}
