package main

import (
	"log"
	"time"

	pingo "github.com/michaeljoyner/pingo/src"
	"github.com/shirou/gopsutil/cpu"
)

func main() {
	dev, err := pingo.New()
	if err != nil {
		log.Fatal(err)
	}
	defer dev.ShutDown()

	red, err := dev.ReqPin(24, pingo.OUTPUT)
	if err != nil {
		log.Fatal(err)
	}

	yellow, err := dev.ReqPin(24, pingo.OUTPUT)
	if err != nil {
		log.Fatal(err)
	}

	green, err := dev.ReqPin(24, pingo.OUTPUT)
	if err != nil {
		log.Fatal(err)
	}

	for range 100 {
		percent, err := cpu.Percent(1, false)
		if err != nil {
			continue
		}

		if percent[0] < 33 {
			green.Set(1)
			yellow.Set(0)
			red.Set(0)
		}

		if percent[0] >= 33 && percent[0] < 67 {
			green.Set(0)
			yellow.Set(1)
			red.Set(0)
		}

		if percent[0] >= 67 {
			green.Set(0)
			yellow.Set(0)
			red.Set(1)
		}

		time.Sleep(1 * time.Second)

	}
}
