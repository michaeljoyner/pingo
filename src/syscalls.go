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
	ID    uint32
	Value uint32
}

type gpioV2LineConfig struct {
	Flags    uint64
	NumAttrs uint32
	Padding  [5]byte
	_        [3]byte
	RawAttrs [80]byte
}

func (cfg *gpioV2LineConfig) SetAttr(index int, attr gpioV2LineAttribute) {
	if index < 0 || index >= 10 {
		panic("SetAttr index out of range")
	}
	offset := index * 8
	binary.LittleEndian.PutUint32(cfg.RawAttrs[offset:], attr.ID)
	binary.LittleEndian.PutUint32(cfg.RawAttrs[offset+4:], attr.Value)
}

type gpioV2LineRequest struct {
	Offsets    [GPIOHANDLES_MAX]uint32
	Consumer   [32]byte
	Config     gpioV2LineConfig
	NumLines   uint32
	EventBufSz uint32
	Padding    [5]byte
	_          [3]byte
	FD         int32
}

type gpioV2LineEvent struct {
	Timestamp_ns uint64
	ID           uint32
	Seqno        uint32
	Line_seqno   uint32
	Padding      [4]byte
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
	fmt.Println("gpioV2LineAttribute:", unsafe.Sizeof(gpioV2LineAttribute{})) // 8
	fmt.Println("gpioV2LineConfig:", unsafe.Sizeof(gpioV2LineConfig{}))       // 100
	fmt.Println("gpioV2LineRequest:", unsafe.Sizeof(gpioV2LineRequest{}))
	var req gpioV2LineRequest
	req.NumLines = 1
	req.Offsets[0] = gpio
	copy(req.Consumer[:], []byte(name))

	// Always set input flag using an attribute
	req.Config.NumAttrs = 2

	// Set edge trigger type
	var edgeValue uint32
	switch edge {
	case 1:
		edgeValue = GPIO_V2_LINE_EDGE_RISING
	case 2:
		edgeValue = GPIO_V2_LINE_EDGE_FALLING
	case 3:
		edgeValue = GPIO_V2_LINE_EDGE_BOTH
	default:
		return -1, fmt.Errorf("invalid edge: %d", edge)
	}

	req.Config.SetAttr(0, gpioV2LineAttribute{
		ID:    GPIO_V2_LINE_ATTR_ID_FLAGS,
		Value: GPIO_V2_LINE_FLAG_INPUT,
	})

	req.Config.SetAttr(1, gpioV2LineAttribute{
		ID:    GPIO_V2_LINE_ATTR_ID_EDGE,
		Value: edgeValue,
	})

	_, _, errno := syscall.Syscall(
		syscall.SYS_IOCTL,
		chip.Fd(),
		uintptr(GPIO_V2_GET_LINE_IOCTL),
		uintptr(unsafe.Pointer(&req)),
	)

	if errno != 0 {
		return -1, fmt.Errorf("ioctl event request failed: %d (%s)", errno, errno.Error())
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

