## Project Overview

This project, `pingo`, is a Go library for controlling General Purpose Input/Output (GPIO) pins on Linux-based systems. It provides a high-level API for interacting with the Linux GPIO character device (`/dev/gpiochip0`), allowing developers to request and control GPIO pins for various hardware projects.

The library is designed to be used in embedded systems like the Raspberry Pi, where direct control of GPIO pins is necessary for interfacing with sensors, actuators, and other electronic components.

The core functionality is implemented in `src/pingo.go`, which provides the main API for GPIO operations. The low-level interaction with the kernel is handled in `src/syscalls.go`, which uses `ioctl` syscalls to communicate with the GPIO driver.

## Building and Running

The project is a Go library and does not have a single main application. To use the library, you can import it into your own Go projects.

The `examples` directory provides sample applications that demonstrate how to use the `pingo` library. To run an example, navigate to its directory and use `go run`:

```sh
cd examples/toggle
go run main.go
```

**Note:** These examples are intended to be run on a system with a GPIO controller, such as a Raspberry Pi.

## Development Conventions

The code follows standard Go conventions. The library is organized into a `src` directory containing the library source code and an `examples` directory for usage examples.

The library uses the `os` and `syscall` packages to interact with the operating system's GPIO device. Error handling is done through Go's standard error mechanism.
