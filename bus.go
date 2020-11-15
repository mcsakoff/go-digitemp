package digitemp

// Conceptual Overview
// -------------------
//
// Properly configured with respect to baud rate, data bits per character, parity and number of stop bits,
// a 115,200 bit per second capable UART provides the input and output timing necessary to implement a 1-Wire master.
// The UART produces the 1-Wire reset pulse, as well as read- and write-time slots. The microprocessor simply puts
// one-byte character codes into the UART transmit register to send a 1-Wire 1 or 0 bit and the UART does the work.
// Conversely, the microprocessor reads single-byte character codes corresponding to a 1 or 0 bit read from a 1-Wire device.
// All 1-Wire bit transfers require the bus master, the UART, to begin the cycle by driving the 1-Wire bus low.
// Therefore, each 1-Wire bit cycle includes a byte transmit and byte receive by the UART. When reading, the received data
// is of interest, when writing, however, the receive byte is discarded. Depending on the UART's read and write first-in,
// first-out (FIFO) buffer depth, the UART can also frame 1-Wire bits into byte values further reducing the processor
// overhead.
//
// For details see:
// Using an UART to Implement a 1-Wire Bus Master (http://www.maximintegrated.com/en/app-notes/index.mvp/id/214)

import (
	"fmt"
	"github.com/tarm/serial"
	"sync"
	"time"
)

type UARTAdapter struct {
	device string
	uart   *serial.Port
	mx     sync.Mutex
}

func NewUartAdapter(device string) (*UARTAdapter, error) {
	adapter := &UARTAdapter{
		device: device,
	}
	config := &serial.Config{
		Name:        adapter.device,
		Baud:        115200,
		ReadTimeout: 3 * time.Second,
		Size:        serial.DefaultSize,
		Parity:      serial.ParityNone,
		StopBits:    serial.Stop1,
	}
	if p, err := serial.OpenPort(config); err != nil {
		return nil, err
	} else {
		adapter.uart = p
	}
	return adapter, nil
}

func (a *UARTAdapter) GetDevice() string {
	return a.device
}

func (a *UARTAdapter) Lock() {
	a.mx.Lock()
}

func (a *UARTAdapter) Unlock() {
	a.mx.Unlock()
}

func (a *UARTAdapter) Reset() error {
	config := &serial.Config{
		Name:        a.device,
		Baud:        9600,
		ReadTimeout: 3 * time.Second,
		Size:        serial.DefaultSize,
		Parity:      serial.ParityNone,
		StopBits:    serial.Stop1,
	}
	if err := a.Close(); err != nil {
		return err
	}

	if p, err := serial.OpenPort(config); err != nil {
		return err
	} else {
		if _, err := p.Write([]byte{0xf0}); err != nil {
			return err
		}
		var buffer [1]byte
		if n, err := p.Read(buffer[0:1]); err != nil {
			return err
		} else {
			if n != 1 {
				return fmt.Errorf("reset: bits expected: 1, got: %d", n)
			}
			if buffer[0] == 0xff {
				return fmt.Errorf("no 1-wire device present")
			} else if buffer[0] < 0x10 || buffer[0] > 0xe0 {
				return fmt.Errorf("presence error 0x%x", buffer[0])
			}
		}
	}

	config.Baud = 115200
	if p, err := serial.OpenPort(config); err != nil {
		return err
	} else {
		a.uart = p
	}
	return nil
}

func (a *UARTAdapter) Clear() error {
	return a.uart.Flush()
}

func (a *UARTAdapter) Close() error {
	if a.uart != nil {
		if err := a.uart.Close(); err != nil {
			return err
		}
	}
	return nil
}

func (a *UARTAdapter) ReadBytes(buffer []byte) (int, error) {
	var err error
	var i int
	size := len(buffer)
	for i = 0; i < size; i++ {
		buffer[i], err = a.ReadByte()
		if err != nil {
			return i, err
		}
	}
	i += 1
	return i, nil
}

func (a *UARTAdapter) WriteBytes(buffer []byte) (int, error) {
	for i, b := range buffer {
		if err := a.WriteByte(b); err != nil {
			return i, err
		}
	}
	return len(buffer), nil
}

//
// Read one bit from serial line.
//
// Writing 0xff starts read time slot. If remote device wants to send 0x0 it will pull the bus low
// and we will read back value < 0xff. Otherwise it is 0x1 was sent.
//
func (a *UARTAdapter) ReadBit() (byte, error) {
	_ = a.uart.Flush()

	if _, err := a.uart.Write([]byte{0xff}); err != nil {
		return 0, err
	}

	var buffer [1]byte
	if n, err := a.uart.Read(buffer[0:1]); err != nil {
		//if err == io.EOF {
		//	return 0xff, nil
		//}
		return 0, err
	} else {
		if n != 1 {
			return 0, fmt.Errorf("bits expected: 1, got: %d", n)
		}
	}

	if buffer[0] == 0xff {
		return 0b1, nil
	} else {
		return 0b0, nil
	}
}

//
// Read one byte from serial line. Same as ReadBit but for 8-bits at once.
//
func (a *UARTAdapter) ReadByte() (byte, error) {
	_ = a.uart.Flush()

	if _, err := a.uart.Write([]byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff}); err != nil {
		return 0, err
	}

	var buffer [8]byte
	if n, err := a.uart.Read(buffer[0:8]); err != nil {
		return 0, err
	} else {
		if n != 8 {
			return 0, fmt.Errorf("bits expected: 8, got: %d", n)
		}
	}

	var data byte = 0
	for n, bit := range buffer {
		if bit == 0xff {
			data += 0x01 << n
		}
	}
	return data, nil
}

//
// Write one bit to serial line.
//
// Writes last bit of the byte. Read-back value shall match the value we write.
// Otherwise someone else was writing to the bus at the same time.
//
func (a *UARTAdapter) WriteBit(data byte) error {
	_ = a.uart.Flush()

	if data%2 == 0 {
		data = 0x00
	} else {
		data = 0xff
	}
	if _, err := a.uart.Write([]byte{data}); err != nil {
		return err
	}

	var buffer [1]byte
	if n, err := a.uart.Read(buffer[0:1]); err != nil {
		return err
	} else {
		if n != 1 {
			return fmt.Errorf("WriteBit: cannot read back")
		}
	}
	if data != buffer[0] {
		return fmt.Errorf("WriteBit: noize detected")
	}
	return nil
}

//
// Write one byte to serial line. Same as WriteBit but for 8-bits at once.
//
func (a *UARTAdapter) WriteByte(data byte) error {
	_ = a.uart.Flush()

	var bits = [8]byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
	for n := 0; n < 8; n++ {
		if (data>>n)%2 != 0 {
			bits[n] = 0xff
		}
	}
	if _, err := a.uart.Write(bits[0:8]); err != nil {
		return err
	}

	var buffer [8]byte
	if n, err := a.uart.Read(buffer[0:8]); err != nil {
		return err
	} else {
		if n != 8 {
			return fmt.Errorf("WriteByte: cannot read back")
		}
	}
	for n, bit := range bits {
		if buffer[n] != bit {
			return fmt.Errorf("WriteByte: noize detected")
		}
	}
	return nil
}
