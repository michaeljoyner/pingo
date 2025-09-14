package pingo

import (
	"errors"
	"os"
)

const (
	INPUT  = GPIOHANDLE_REQUEST_INPUT
	OUTPUT = GPIOHANDLE_REQUEST_OUTPUT

	EDGE_RISING  = uint8(1)
	EDGE_FALLING = uint8(2)
	EDGE_BOTH    = uint8(3)
)

type Device struct {
	chip  os.File
	Name  string
	lines map[int]Line
}

type Line struct {
	Fd          int
	Mode        uint32
	isInterrupt bool
	file        *os.File
}

func New() (Device, error) {
	file, err := os.OpenFile("/dev/gpiochip0", os.O_RDWR, 0)

	if err != nil {
		return Device{}, err
	}

	return Device{
		chip:  *file,
		Name:  "pingo-gpio",
		lines: make(map[int]Line),
	}, nil
}

func (d *Device) ShutDown() {
	for _, line := range d.lines {
		line.file.Close()
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
	file := os.NewFile(uintptr(fd), "gpio")
	line := Line{Fd: fd, Mode: mode, isInterrupt: false, file: file}
	d.lines[number] = line
	return line, nil
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
	file := os.NewFile(uintptr(fd), "gpio")
	line := Line{Fd: fd, Mode: INPUT, isInterrupt: true, file: file}
	d.lines[number] = line
	return line, nil
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
				edge, err := waitForInterrupt(l.file)
				if err != nil {
					continue
				}
				detections <- edge
			}
		}
	}()

	return detections, nil
}
