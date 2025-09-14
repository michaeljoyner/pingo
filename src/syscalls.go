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

type gpioV2LineAttribute struct {
	ID      uint32
	padding uint32 // must be zero
	Value   uint64
}

type gpioV2LineConfigAttribute struct {
	Attr gpioV2LineAttribute
	Mask uint64
}

type gpioV2LineConfig struct {
	Flags    uint64
	NumAttrs uint32
	_        [5]uint32 // padding[5]
	Attrs    [GPIO_V2_LINE_NUM_ATTRS_MAX]gpioV2LineConfigAttribute
}

type gpioV2LineRequest struct {
	Offsets         [GPIO_V2_LINES_MAX]uint32
	Consumer        [GPIO_MAX_NAME_SIZE]byte
	Config          gpioV2LineConfig
	NumLines        uint32
	EventBufferSize uint32
	padding         [5]uint32
	FD              int32
}

type gpioV2LineEvent struct {
	Timestamp_ns uint64
	ID           uint32
	Seqno        uint32
	LineSeqno    uint32
	_            [4]byte
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
	var req gpioV2LineRequest

	// Clear everything initially (zero-initialized by Go on var declaration)

	req.Offsets[0] = gpio
	req.NumLines = 1

	copy(req.Consumer[:], name)
	if len(name) < len(req.Consumer) {
		req.Consumer[len(name)] = 0
	}

	// Set configuration flags
	req.Config.Flags = GPIO_V2_LINE_FLAG_INPUT

	// Prepare attributes
	// attribute 0: flags input
	req.Config.Attrs[0] = gpioV2LineConfigAttribute{
		Attr: gpioV2LineAttribute{
			ID:      GPIO_V2_LINE_ATTR_ID_FLAGS,
			padding: 0,
			Value:   uint64(GPIO_V2_LINE_FLAG_INPUT),
		},
		// Mask: for line index 0
		Mask: 1 << 0,
	}

	// attribute 1: edge detection
	var edgeVal uint64
	switch edge {
	case 1:
		edgeVal = GPIO_V2_LINE_EDGE_RISING
	case 2:
		edgeVal = GPIO_V2_LINE_EDGE_FALLING
	case 3:
		edgeVal = GPIO_V2_LINE_EDGE_BOTH
	default:
		return -1, fmt.Errorf("invalid edge: %d", edge)
	}

	req.Config.Attrs[1] = gpioV2LineConfigAttribute{
		Attr: gpioV2LineAttribute{
			ID:      GPIO_V2_LINE_ATTR_ID_EDGE,
			padding: 0,
			Value:   edgeVal,
		},
		Mask: 1 << 0,
	}

	req.Config.NumAttrs = 2

	req.EventBufferSize = 0 // or pass a non-zero if you want

	// Debug: check sizes
	fmt.Println("sizeof gpioV2LineAttribute:", unsafe.Sizeof(gpioV2LineAttribute{}))             // should be 16
	fmt.Println("sizeof gpioV2LineConfigAttribute:", unsafe.Sizeof(gpioV2LineConfigAttribute{})) // should be 24
	fmt.Println("sizeof gpioV2LineConfig:", unsafe.Sizeof(gpioV2LineConfig{}))                   // should be 272
	fmt.Println("sizeof gpioV2LineRequest:", unsafe.Sizeof(req))                                 // should be 592

	// Perform the ioctl
	_, _, errno := syscall.Syscall(
		syscall.SYS_IOCTL,
		chip.Fd(),
		uintptr(GPIO_V2_GET_LINE_IOCTL),
		uintptr(unsafe.Pointer(&req)),
	)
	if errno != 0 {
		return -1, fmt.Errorf("ioctl failed: %d (%s)", errno, errno.Error())
	}

	return int(req.FD), nil
}

func waitForInterrupt(file *os.File) (uint8, error) {
	var event gpioV2LineEvent
	err := binary.Read(file, binary.LittleEndian, &event)
	if err != nil {
		return 0, fmt.Errorf("failed to read event: %v", err)
	}

	switch event.ID {
	case GPIO_V2_LINE_EVENT_RISING_EDGE:
		return 1, nil
	case GPIO_V2_LINE_EVENT_FALLING_EDGE:
		return 2, nil
	default:
		return 3, nil
	}
}

