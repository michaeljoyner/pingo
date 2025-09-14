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

type gpioV2LineAttributeV2 struct {
	ID    uint32
	Value uint32
}

func buildLineConfigBuffer(attrs []gpioV2LineAttribute) ([]byte, error) {
	const size = 100
	buf := make([]byte, size)

	var flags uint64 = GPIO_V2_LINE_FLAG_INPUT
	binary.LittleEndian.PutUint64(buf[0:], flags)

	numAttrs := uint32(len(attrs))
	binary.LittleEndian.PutUint32(buf[8:], numAttrs)

	// zero padding
	for i := 12; i < 16; i++ {
		buf[i] = 0
	}

	for i, attr := range attrs {
		if i >= 10 {
			return nil, fmt.Errorf("too many attributes")
		}
		offset := 16 + i*8
		binary.LittleEndian.PutUint32(buf[offset:], attr.ID)
		binary.LittleEndian.PutUint32(buf[offset+4:], attr.Value)
	}

	return buf, nil
}

func buildLineConfigBufferV2(attrs []gpioV2LineAttributeV2) ([]byte, error) {
	// Total size of struct gpio_v2_line_config
	const configSize = 8 + 4 + 20 + GPIO_V2_LINE_NUM_ATTRS_MAX*24
	buf := make([]byte, configSize)

	// 0..7: flags (u64)
	var flags uint64 = GPIO_V2_LINE_FLAG_INPUT
	binary.LittleEndian.PutUint64(buf[0:], flags)

	// 8..11: num_attrs (u32)
	numAttrs := uint32(len(attrs))
	if numAttrs > GPIO_V2_LINE_NUM_ATTRS_MAX {
		return nil, fmt.Errorf("too many attributes: %d", numAttrs)
	}
	binary.LittleEndian.PutUint32(buf[8:], numAttrs)

	// 12..31: padding[5] (5 × u32) → 20 bytes => already zero via make

	// Attributes start at offset 32
	for i, attr := range attrs {
		base := 32 + i*24
		// attr.id, 4 bytes
		binary.LittleEndian.PutUint32(buf[base+0:], attr.ID)
		// padding, 4 bytes at base+4 -> leave 0
		// union field: u64 at base+8
		binary.LittleEndian.PutUint64(buf[base+8:], uint64(attr.Value))
		// mask: u64 at base+16
		// e.g. if your request has 1 line and you want this attr to apply to that line:
		mask := uint64(1) << uint(i) // or other mapping; for single line, maybe just 1
		binary.LittleEndian.PutUint64(buf[base+16:], mask)
	}

	// Unused attributes will stay zero (since make filled buf with zeros)
	return buf, nil
}

type gpioV2LineRequest struct {
	Offsets    [GPIOHANDLES_MAX]uint32
	Consumer   [32]byte
	ConfigBuf  [100]byte // raw serialized gpioV2LineConfig
	NumLines   uint32
	EventBufSz uint32
	Padding    [8]byte // to ensure proper alignment
	FD         int32
}

type gpioV2LineRequestV2 struct {
	Offsets    [GPIO_V2_LINES_MAX]uint32
	Consumer   [GPIO_MAX_NAME_SIZE]byte
	ConfigBuf  []byte // we'll handle as raw bytes
	NumLines   uint32
	EventBufSz uint32
	Padding    [5]uint32 // u32 padding[5]
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
	// Prepare attributes
	attrs := []gpioV2LineAttributeV2{
		{ID: GPIO_V2_LINE_ATTR_ID_FLAGS, Value: GPIO_V2_LINE_FLAG_INPUT},
	}
	var edgeVal uint32
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
	attrs = append(attrs, gpioV2LineAttributeV2{ID: GPIO_V2_LINE_ATTR_ID_OUTPUT_VALUES /* or EDGE_ATTR_ID if exists */, Value: edgeVal})

	// Build config buffer
	configBuf, err := buildLineConfigBufferV2(attrs)
	if err != nil {
		return -1, err
	}

	// Now build the raw request buffer
	// calculate size
	const configSize = 8 + 4 + 20 + GPIO_V2_LINE_NUM_ATTRS_MAX*24
	// request size:
	requestSize := GPIO_V2_LINES_MAX*4 + GPIO_MAX_NAME_SIZE + configSize + 4 + 4 + 5*4 + 4

	// allocate raw buffer
	raw := make([]byte, requestSize)

	offset := 0
	// Offsets[0]
	binary.LittleEndian.PutUint32(raw[offset:], gpio)
	offset += GPIO_V2_LINES_MAX * 4

	// Consumer (name, NUL terminated)
	copy(raw[offset:], []byte(name))
	// If name shorter than consumer size, NUL pad
	// name size = GPIO_MAX_NAME_SIZE
	offset += GPIO_MAX_NAME_SIZE

	// Config struct
	copy(raw[offset:], configBuf)
	offset += configSize

	// num_lines (u32)
	binary.LittleEndian.PutUint32(raw[offset:], 1)
	offset += 4

	// event_buffer_size (u32)
	binary.LittleEndian.PutUint32(raw[offset:], 0) // or non-zero if you want event buffering
	offset += 4

	// padding[5] (5 × u32) = 20 bytes
	// raw[offset:offset+20] already zero
	offset += 5 * 4

	// fd (int32)
	// this is an output; we don’t set before ioctl
	offset += 4

	// Now do ioctl
	_, _, errno := syscall.Syscall(
		syscall.SYS_IOCTL,
		chip.Fd(),
		uintptr(GPIO_V2_GET_LINE_IOCTL),
		uintptr(unsafe.Pointer(&raw[0])),
	)
	if errno != 0 {
		return -1, fmt.Errorf("GPIO_V2_GET_LINE_IOCTL failed: %d (%s)", errno, errno.Error())
	}

	// After call, raw buffer’s fd field is set by kernel: read back from the buffer
	// extract fd from raw[offset_of_fd] (you know where that was)
	// e.g.:
	fdOffset := GPIO_V2_LINES_MAX*4 + GPIO_MAX_NAME_SIZE + configSize + 4 + 4 + 5*4
	fd := int32(binary.LittleEndian.Uint32(raw[fdOffset : fdOffset+4]))
	if fd < 0 {
		return -1, fmt.Errorf("invalid fd returned: %d", fd)
	}
	return int(fd), nil
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

