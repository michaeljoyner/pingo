package main

import (
	"log"
	"os"
	"syscall"
	"time"

	pingo "github.com/michaeljoyner/pingo/src"
)

func main() {
	dev, err := pingo.New()
	if err != nil {
		log.Fatalf("Failed to open gpiochip: %v", err)
	}
	defer dev.ShutDown()

	// BCM GPIO 17
	gpioNum := uint32(17)
	fd, err := pingo.RequestLine(chip, gpioNum, true)
	if err != nil {
		log.Fatalf("Failed to request GPIO line: %v", err)
	}
	defer syscall.Close(fd)

	// Toggle pin
	for range 10 {
		pingo.SetLineValue(fd, 1)
		time.Sleep(500 * time.Millisecond)
		pingo.SetLineValue(fd, 0)
		time.Sleep(500 * time.Millisecond)
	}
}
