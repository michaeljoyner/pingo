package main

import (
	"log"
	"time"

	pingo "github.com/michaeljoyner/pingo/src"
)

func main() {
	dev, err := pingo.New()
	if err != nil {
		log.Fatalf("Failed to open gpiochip: %v", err)
	}
	defer dev.ShutDown()

	light, err := dev.ReqPin(17, pingo.OUTPUT)
	if err != nil {
		log.Fatal(err)
	}

	// Toggle pin
	for range 10 {
		light.Set(1)
		time.Sleep(500 * time.Millisecond)
		light.Set(0)
		time.Sleep(500 * time.Millisecond)
	}
}
