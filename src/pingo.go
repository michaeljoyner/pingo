package pingo

import (
	"errors"
	"os"
	"syscall"
)

const (
	INPUT  = GPIOHANDLE_REQUEST_INPUT
	OUTPUT = GPIOHANDLE_REQUEST_OUTPUT

	EDGE_RISING  = 1
	EDGE_FALLING = 2
	EDGE_BOTH    = 3
)

type Device struct {
	chip  os.File
	Name  string
	lines map[int]int
}

type Line struct {
	Fd          int
	Mode        uint32
	isInterrupt bool
}

func New() (Device, error) {
	file, err := os.OpenFile("/dev/gpiochip0", os.O_RDWR, 0)

	if err != nil {
		return Device{}, err
	}

	return Device{
		chip: *file,
		Name: "pingo-gpio",
	}, nil
}

func (d *Device) ShutDown() {
	for _, fd := range d.lines {
		syscall.Close(fd)
	}

	d.chip.Close()

}

func (d *Device) ReqPin(number int, mode uint32) (Line, error) {
	_, used := d.lines[number]
	if used {
		return Line{}, errors.New("line already in use")
	}
	fd, err := requestLine(&d.chip, uint32(number), mode == OUTPUT, d.Name)
	if err != nil {
		return Line{}, nil
	}
	d.lines[number] = fd
	return Line{Fd: fd, Mode: mode, isInterrupt: false}, nil
}

func (d *Device) ReqIRQ(number int, direction uint8) (Line, error) {
	_, used := d.lines[number]
	if used {
		return Line{}, errors.New("line already in use")
	}
	fd, err := requestInterruptLine(&d.chip, uint32(number), direction, d.Name)
	if err != nil {
		return Line{}, err
	}
	return Line{Fd: fd, Mode: INPUT, isInterrupt: true}, nil
}

func (l Line) Set(value uint8) error {
	err := setLineValue(l.Fd, value)
	if err != nil {
		return err
	}
	return nil
}

func (l Line) Get() (uint8, error) {
	return getLineValue(l.Fd)
}

func (l Line) Listen(quit <-chan struct{}) (<-chan uint8, error) {
	if !l.isInterrupt {
		return make(<-chan uint8), errors.New("cannot listen on non-IRQ configured pin")
	}
	detections := make(chan uint8)
	go func() {
		defer close(detections)
		for {
			select {
			case <-quit:
				return
			default:
				edge, err := waitForInterrupt(l.Fd)
				if err != nil {
					continue
				}
				detections <- edge
			}
		}
	}()

	return detections, nil
}
