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
	"errors"
	"fmt"
	"go.bug.st/serial"
	"sync"
)

type UARTAdapter struct {
	device string
	uart   serial.Port
	mode   serial.Mode
	mx     sync.Mutex
}

func NewUartAdapter(device string) (*UARTAdapter, error) {
	adapter := &UARTAdapter{
		device: device,
		mode: serial.Mode{
			BaudRate: 115200,
			DataBits: 8,
			Parity:   serial.NoParity,
			StopBits: serial.OneStopBit,
		},
	}
	if p, err := serial.Open(device, &adapter.mode); err != nil {
		return nil, err
	} else {
		adapter.uart = p
		_ = p.SetDTR(true)  // TODO: check for error
	}
	return adapter, nil
}

// Get serial port name.
func (a *UARTAdapter) GetDevice() string {
	return a.device
}

// Get ROM of a single device connected to the bus.
// This command can only be used when there is one device on the bus.
func (a *UARTAdapter) GetSingleROM() (*ROM, error) {
	a.lock()
	defer a.unlock()

	return a.readROM()
}

// Get ROM of devices connected to the bus.
func (a *UARTAdapter) GetConnectedROMs() ([]*ROM, error) {
	a.lock()
	defer a.unlock()

	return a.searchROM(false)
}

// Get ROM of devices with a set alarm flag.
func (a *UARTAdapter) GetROMsWithAlarm() ([]*ROM, error) {
	a.lock()
	defer a.unlock()

	return a.searchROM(true)
}

// Check device is connected to the bus
func (a *UARTAdapter) IsConnected(rom *ROM) (bool, error) {
	a.lock()
	defer a.unlock()

	return a.isConnected(rom)
}

// Close serial port.
func (a *UARTAdapter) Close() error {
	a.lock()
	defer a.unlock()

	return a.close()
}

func (a *UARTAdapter) lock() {
	a.mx.Lock()
}

func (a *UARTAdapter) unlock() {
	a.mx.Unlock()
}

// Send Reset impulse and check device's presence.
func (a *UARTAdapter) reset() error {
	a.mode.BaudRate = 9600
	if err := a.uart.SetMode(&a.mode); err != nil {
		return err
	}

	if err := a.clear(); err != nil {
		return err
	}

	var pulse = func() error {
		if n, err := a.uart.Write([]byte{0xf0}); err != nil {
			return err
		} else{
			if n != 1 {
				return fmt.Errorf("failed to write reset pulse")
			}
		}
		var buffer [1]byte
		if n, err := a.uart.Read(buffer[0:1]); err != nil {
			return err
		} else {
			if n != 1 {
				return fmt.Errorf("failed to read back reset pulse")
			}
			if buffer[0] & 0xf != 0x0 {
				return fmt.Errorf("reset pulse error 0x%x", buffer[0])
			}
			if buffer[0] >> 4 == 0xf {
				return fmt.Errorf("no 1-wire device present")
			}
		}
		return nil
	}
	pulseErr := pulse()

	a.mode.BaudRate = 115200
	if err := a.uart.SetMode(&a.mode); err != nil {
		return err
	}

	return pulseErr
}

// Discards data in input/output buffers
func (a *UARTAdapter) clear() error {
	if err := a.uart.ResetOutputBuffer(); err != nil {
		return err
	}
	if err := a.uart.ResetInputBuffer(); err != nil {
		return err
	}
	return nil
}

// Close serial port.
func (a *UARTAdapter) close() error {
	if a.uart != nil {
		if err := a.uart.Close(); err != nil {
			return err
		}
	}
	return nil
}

func (a *UARTAdapter) readBytes(buffer []byte) (int, error) {
	var err error
	var i int
	size := len(buffer)
	for i = 0; i < size; i++ {
		buffer[i], err = a.readByte()
		if err != nil {
			return i, err
		}
	}
	return i, nil
}

// Read one byte from serial line. Same as ReadBit but for 8-bits at once.
func (a *UARTAdapter) readByte() (byte, error) {
	_ = a.clear()

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

// Read one bit from serial line.
// Writing 0xff starts read time slot. If remote device wants to send 0x0 it will pull the bus low
// and we will read back value < 0xff. Otherwise it is 0x1 was sent.
func (a *UARTAdapter) readBit() (byte, error) {
	_ = a.clear()

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

func (a *UARTAdapter) writeBytes(buffer []byte) (int, error) {
	for i, b := range buffer {
		if err := a.writeByte(b); err != nil {
			return i, err
		}
	}
	return len(buffer), nil
}

// Write one byte to serial line. Same as WriteBit but for 8-bits at once.
func (a *UARTAdapter) writeByte(data byte) error {
	_ = a.clear()

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
			return fmt.Errorf("WriteByte: noize detected(got: 0x%02x, expected: 0x%02x)", buffer[n], bit)
		}
	}
	return nil
}

// Write one bit to serial line.
// Writes last bit of the byte. Read-back value shall match the value we write.
// Otherwise someone else was writing to the bus at the same time.
func (a *UARTAdapter) writeBit(data byte) error {
	_ = a.clear()

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
// READ ROM [33h]
//
// This command can only be used when there is one device on the bus. It allows the bus driver to read the
// device's 64-bit ROM code without using the Search ROM procedure. If this command is used when there
// is more than one device present on the bus, a data collision will occur when all of the devices attempt
// to respond at the same time.
//
func (a *UARTAdapter) readROM() (*ROM, error) {
	if err := a.reset(); err != nil {
		return nil, err
	}
	if err := a.writeByte(0x33); err != nil {
		return nil, err
	}
	var rom = new(ROM)
	if _, err := a.readBytes(rom.Code[0:8]); err != nil {
		return nil, err
	}
	if !rom.IsValid() {
		return nil, errors.New("crc error")
	}
	return rom, nil
}

//
// MATCH ROM [55h]
//
// The match ROM command allows to address a specific device on a multidrop or single-drop bus.
// Only the device that exactly matches the 64-bit ROM code sequence will respond to the function command
// issued by the bus driver; all other devices on the bus will wait for a reset pulse.
//
func (a *UARTAdapter) matchROM(rom *ROM) error {
	if err := a.reset(); err != nil {
		return err
	}
	if err := a.writeByte(0x55); err != nil {
		return err
	}
	if _, err := a.writeBytes(rom.Code[0:8]); err != nil {
		return err
	}
	return nil
}

// The bus driver can use this command to address all devices on the bus simultaneously without sending out
// any ROM code information.
func (a *UARTAdapter) skipROM() error {
	if err := a.reset(); err != nil {
		return err
	}
	if err := a.writeByte(0xcc); err != nil {
		return err
	}
	return nil
}

//
// SEARCH ROM [F0h]
// The bus driver learns the ROM codes through a process of elimination that requires it to perform
// a Search ROM cycle as many times as necessary to identify all of the devices.
//
// ALARM SEARCH [ECh]
// The operation of this command is identical to the operation of the Search ROM command except that
// only devices with a set alarm flag will respond.
//
func (a *UARTAdapter) searchROM(WithAlarm bool) ([]*ROM, error) {
	var command byte
	if WithAlarm {
		command = 0xec
	} else {
		command = 0xf0
	}

	var complete = make([]*ROM, 0)
	var partials = make([][]byte, 0)
	var current []byte = nil
	for {
		// send search command
		if err := a.reset(); err != nil {
			return nil, err
		}
		if err := a.writeByte(command); err != nil {
			return nil, err
		}
		// send known bits
		for _, bit := range current {
			if _, err := a.readBit(); err != nil { // skip bitN
				return nil, err
			}
			if _, err := a.readBit(); err != nil { // skip complement of bitN
				return nil, err
			}
			if err := a.writeBit(bit); err != nil {
				return nil, err
			}
		}
		// read rest of the bits
		for len(current) < 64 {
			var b1, b2 byte
			var err error
			if b1, err = a.readBit(); err != nil {
				return nil, err
			}
			if b2, err = a.readBit(); err != nil {
				return nil, err
			}
			if b1 != b2 {
				// all devices have this bit set to 0 or 1
				current = append(current, b1)
				if err = a.writeBit(b1); err != nil {
					return nil, err
				}
			} else if b1 == b2 && b1 == 0b0 {
				// there are two or more devices on the bus with bit 0 and 1 in this position
				// save version with 1 as possible rom ...
				r := make([]byte, len(current))
				copy(r, current)
				r = append(r, 0b1)
				partials = append(partials, r)
				// ... and proceed with 0
				current = append(current, 0b0)
				if err = a.writeBit(0b0); err != nil {
					return nil, err
				}
			} else { // b1 == b2 == 1
				if WithAlarm {
					// in alarm search that means there is no more alarming devices
					break
				} else {
					return nil, errors.New("search command got wrong bits (two sequential 0b1)")
				}
			}
		}
		complete = append(complete, newRomFromBits(current))
		if len(partials) == 0 {
			break
		}
		current = partials[0]
		partials = partials[1:]
	}
	return complete, nil
}

func (a *UARTAdapter) isConnected(rom *ROM) (bool, error) {
	if err := a.reset(); err != nil {
		return false, err
	}
	if err := a.writeByte(0xf0); err != nil {
		return false, err
	}
	for _, bit := range rom.toBits() {
		b1, _ := a.readBit()
		b2, _ := a.readBit()
		if b1 == b2 && b1 == 0b1 {
			return false, nil
		}
		if err := a.writeBit(bit); err != nil {
			return false, err
		}
	}
	return true, nil
}
