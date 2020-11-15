package digitemp

import "errors"

type AddressableDevice struct {
	bus *UARTAdapter
}

func NewAddressableDevice(port *UARTAdapter) *AddressableDevice {
	return &AddressableDevice{
		bus: port,
	}
}

func (d *AddressableDevice) GetBusDeviceName() string {
	return d.bus.device
}

func (d *AddressableDevice) GetSingleROM() (*ROM, error) {
	d.bus.Lock()
	defer d.bus.Unlock()

	return d.readROM()
}

func (d *AddressableDevice) GetConnectedROMs() ([]*ROM, error) {
	d.bus.Lock()
	defer d.bus.Unlock()

	return d.searchROM(false)
}

func (d *AddressableDevice) GetROMsWithAlarm() ([]*ROM, error) {
	d.bus.Lock()
	defer d.bus.Unlock()

	return d.searchROM(true)
}

func (d *AddressableDevice) IsConnected(rom *ROM) (bool, error) {
	d.bus.Lock()
	defer d.bus.Unlock()

	return d.isConnected(rom)
}

//
// READ ROM [33h]
//
// This command can only be used when there is one device on the bus. It allows the bus driver to read the
// device's 64-bit ROM code without using the Search ROM procedure. If this command is used when there
// is more than one device present on the bus, a data collision will occur when all of the devices attempt
// to respond at the same time.
//
func (d *AddressableDevice) readROM() (*ROM, error) {
	if err := d.bus.Reset(); err != nil {
		return nil, err
	}
	if err := d.bus.WriteByte(0x33); err != nil {
		return nil, err
	}
	var rom = new(ROM)
	if _, err := d.bus.ReadBytes(rom.Code[0:8]); err != nil {
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
func (d *AddressableDevice) matchROM(rom *ROM) error {
	if err := d.bus.Reset(); err != nil {
		return err
	}
	if err := d.bus.WriteByte(0x55); err != nil {
		return err
	}
	if _, err := d.bus.WriteBytes(rom.Code[0:8]); err != nil {
		return err
	}
	return nil
}

//
// The bus driver can use this command to address all devices on the bus simultaneously without sending out
// any ROM code information.
//
func (d *AddressableDevice) skipROM() error {
	if err := d.bus.Reset(); err != nil {
		return err
	}
	if err := d.bus.WriteByte(0xcc); err != nil {
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
func (d *AddressableDevice) searchROM(WithAlarm bool) ([]*ROM, error) {
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
		if err := d.bus.Reset(); err != nil {
			return nil, err
		}
		if err := d.bus.WriteByte(command); err != nil {
			return nil, err
		}
		// send known bits
		for _, bit := range current {
			if _, err := d.bus.ReadBit(); err != nil { // skip bitN
				return nil, err
			}
			if _, err := d.bus.ReadBit(); err != nil { // skip complement of bitN
				return nil, err
			}
			if err := d.bus.WriteBit(bit); err != nil {
				return nil, err
			}
		}
		// read rest of the bits
		for len(current) < 64 {
			var b1, b2 byte
			var err error
			if b1, err = d.bus.ReadBit(); err != nil {
				return nil, err
			}
			if b2, err = d.bus.ReadBit(); err != nil {
				return nil, err
			}
			if b1 != b2 {
				// all devices have this bit set to 0 or 1
				current = append(current, b1)
				if err = d.bus.WriteBit(b1); err != nil {
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
				if err = d.bus.WriteBit(0b0); err != nil {
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

func (d *AddressableDevice) isConnected(rom *ROM) (bool, error) {
	if err := d.bus.Reset(); err != nil {
		return false, err
	}
	if err := d.bus.WriteByte(0xf0); err != nil {
		return false, err
	}
	for _, bit := range rom.toBits() {
		b1, _ := d.bus.ReadBit()
		b2, _ := d.bus.ReadBit()
		if b1 == b2 && b1 == 0b1 {
			return false, nil
		}
		if err := d.bus.WriteBit(bit); err != nil {
			return false, err
		}
	}
	return true, nil
}
