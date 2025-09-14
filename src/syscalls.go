package pingo

import (
	"encoding/binary"
	"fmt"
	"os"
	"syscall"
	"unsafe"
)

type GPIOHandleRequest struct {
	LineOffsets   [GPIOHANDLES_MAX]uint32
	Flags         uint32
	DefaultValues [GPIOHANDLES_MAX]uint8
	ConsumerLabel [32]byte
	Lines         uint32
	FD            int32
	Padding       [4]byte
}

type GPIOHandleData struct {
	Values [GPIOHANDLES_MAX]uint8
}

type GPIOEventRequest struct {
	LineOffset    uint32
	HandleFlags   uint32
	EventFlags    uint32
	ConsumerLabel [32]byte
	FD            int32
	_             [4]byte
}

type GPIOEventData struct {
	Timestamp uint64
	ID        uint32
	_         uint32 // padding
}

func requestLine(chip *os.File, gpio uint32, output bool, name string) (int, error) {
	var req GPIOHandleRequest
	req.LineOffsets[0] = gpio
	req.Lines = 1

	if output {
		req.Flags = GPIOHANDLE_REQUEST_OUTPUT
		req.DefaultValues[0] = 0
	} else {
		req.Flags = GPIOHANDLE_REQUEST_INPUT
	}

	copy(req.ConsumerLabel[:], []byte(name))

	_, _, errno := syscall.Syscall(
		syscall.SYS_IOCTL,
		chip.Fd(),
		uintptr(GPIO_GET_LINEHANDLE_IOCTL),
		uintptr(unsafe.Pointer(&req)),
	)

	if errno != 0 {
		return -1, fmt.Errorf("ioctl failed: %v", errno)
	}

	return int(req.FD), nil
}

func setLineValue(fd int, value uint8) error {
	var data GPIOHandleData
	data.Values[0] = value

	_, _, errno := syscall.Syscall(
		syscall.SYS_IOCTL,
		uintptr(fd),
		uintptr(GPIOHANDLE_SET_LINE_VALUES_IOCTL),
		uintptr(unsafe.Pointer(&data)),
	)

	if errno != 0 {
		return fmt.Errorf("set value ioctl failed: %v", errno)
	}

	return nil
}

func getLineValue(fd int) (uint8, error) {
	var data GPIOHandleData

	_, _, errno := syscall.Syscall(
		syscall.SYS_IOCTL,
		uintptr(fd),
		uintptr(GPIOHANDLE_GET_LINE_VALUES_IOCTL),
		uintptr(unsafe.Pointer(&data)),
	)

	if errno != 0 {
		return 0, fmt.Errorf("get value ioctl failed: %v", errno)
	}

	return data.Values[0], nil
}

func requestInterruptLine(chip *os.File, gpio uint32, edge uint8, name string) (int, error) {
	var req GPIOEventRequest
	req.LineOffset = gpio
	req.HandleFlags = GPIOHANDLE_REQUEST_INPUT

	switch edge {
	case 1:
		req.EventFlags = GPIOEVENT_REQUEST_RISING_EDGE
	case 2:
		req.EventFlags = GPIOEVENT_REQUEST_FALLING_EDGE
	case 3:
		req.EventFlags = GPIOEVENT_REQUEST_BOTH_EDGES
	default:
		return -1, fmt.Errorf("invalid edge: %d", edge)
	}

	copy(req.ConsumerLabel[:], []byte(name))

	_, _, errno := syscall.Syscall(
		syscall.SYS_IOCTL,
		chip.Fd(),
		uintptr(GPIO_GET_LINEEVENT_IOCTL),
		uintptr(unsafe.Pointer(&req)),
	)

	if errno != 0 {
		return -1, fmt.Errorf("ioctl event request failed: %v", errno)
	}

	return int(req.FD), nil
}

func waitForInterrupt(fd int) (uint8, error) {
	file := os.NewFile(uintptr(fd), "gpio")
	defer file.Close()

	var event GPIOEventData
	err := binary.Read(file, binary.LittleEndian, &event)
	if err != nil {
		return 0, fmt.Errorf("failed to read event: %v", err)
	}

	switch event.ID {
	case GPIOEVENT_EVENT_RISING_EDGE:
		return 1, nil
	case GPIOEVENT_EVENT_FALLING_EDGE:
		return 2, nil
	default:
		return 3, nil
	}
}
