package digitemp

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"time"
)

const (
	BSResolution9bits = 0x0

	BS18S20ResolutionExtended = 0x1

	BS18B20Resolution9bits  = 0x0
	BS18B20Resolution10bits = 0x1
	BS18B20Resolution11bits = 0x2
	BS18B20Resolution12bits = 0x3
)

type TemperatureSensor struct {
	AddressableDevice
	rom           *ROM
	familyCode    byte
	singleMode    bool
	parasiticMode bool
	resolution    byte
	description   string
	precision     string
	tConv         time.Duration // temperature conversion time
	tRW           time.Duration // eeprom write time
}

//
// Create new temperature sensor instance.
//
// If rom is nil, it will read ROM code from the bus. It works in case of only one sensor connected.
// If required is false, it will not fail with error if the sensor doesn't respond during initialization.
//
func NewTemperatureSensor(bus *UARTAdapter, rom *ROM, required bool) (*TemperatureSensor, error) {
	s := &TemperatureSensor{
		AddressableDevice: AddressableDevice{
			bus: bus,
		},
		rom:        rom,
		resolution: BSResolution9bits,
		tConv:      750 * time.Millisecond,
		tRW:        10 * time.Millisecond,
	}

	s.bus.Lock()
	defer s.bus.Unlock()

	if s.rom == nil {
		s.singleMode = true
		if rom, err := s.readROM(); err != nil {
			if required {
				return nil, errors.New("cannot read sensor's ROM code")
			}
		} else {
			s.rom = rom
		}
	} else {
		s.singleMode = false
		if online, err := s.isConnected(s.rom); err != nil {
			return nil, err
		} else {
			if required && !online {
				return nil, fmt.Errorf("device with ROM %s not found", s.rom)
			}
		}
	}

	if pm, err := s.inParasiticMode(); err != nil {
		return nil, err
	} else {
		s.parasiticMode = pm
	}
	s.familyCode = s.rom.Code[0]

	switch s.familyCode {
	case 0x00:
		s.description = "Unidentified device"
	case 0x10:
		s.description = "DS18S20 - High-precision Digital Thermometer"
	case 0x22:
		s.description = "DS1822 - Econo Digital Thermometer"
	case 0x28:
		s.description = "DS18B20 - Programmable Resolution Digital Thermometer"
	}

	switch s.familyCode {
	case 0x10:
		if s.resolution == BSResolution9bits {
			s.precision = "9 bits"
		} else {
			s.precision = "extended"
		}
	case 0x22, 0x28:
		if sp, err := s.readScratchpad(); err != nil {
			return nil, err
		} else {
			s.resolution = (sp[4] >> 5) & 0b11
		}
		s.tConv = time.Millisecond * (750 / (8 >> s.resolution))
		s.precision = fmt.Sprintf("%d bits", 9+s.resolution)
	default:
		s.precision = "unknown"
	}
	return s, nil
}

func (s *TemperatureSensor) GetROM() *ROM {
	return s.rom
}

func (s *TemperatureSensor) GetFamilyCode() byte {
	return s.familyCode
}

func (s *TemperatureSensor) GetName() string {
	return s.description
}

func (s *TemperatureSensor) GetPrecision() string {
	return s.precision
}

func (s *TemperatureSensor) IsParasiticMode() bool {
	return s.parasiticMode
}

func (s *TemperatureSensor) SaveEEPROM() error {
	s.bus.Lock()
	defer s.bus.Unlock()

	if err := s.copyScratchpad(); err != nil {
		return err
	}
	return nil
}

func (s *TemperatureSensor) LoadEEPROM() error {
	s.bus.Lock()
	defer s.bus.Unlock()

	if err := s.recallScratchpad(); err != nil {
		return err
	}
	return nil
}

//
// Returns temperature * 100 ºC
//
func (s *TemperatureSensor) GetTemperature() (int, error) {
	s.bus.Lock()
	defer s.bus.Unlock()

	if err := s.convertT(); err != nil {
		return 0, err
	}
	if sp, err := s.readScratchpad(); err != nil {
		return 0, err
	} else {
		return s.calcTemperature(sp) / 100, nil
	}
}

//
// Returns temperature ºC
//
func (s *TemperatureSensor) GetTemperatureFloat() (float32, error) {
	if t, err := s.GetTemperature(); err != nil {
		return 0, err
	} else {
		return float32(t) / 100.0, nil
	}
}

func (s *TemperatureSensor) GetAlarms() (int8, int8, error) {
	s.bus.Lock()
	defer s.bus.Unlock()

	if sp, err := s.readScratchpad(); err != nil {
		return 0, 0, err
	} else {
		return int8(sp[2]), int8(sp[3]), nil
	}
}

func (s *TemperatureSensor) SetAlarms(high int8, low int8) error {
	s.bus.Lock()
	defer s.bus.Unlock()

	var scratchpad []byte
	if sp, err := s.readScratchpad(); err != nil {
		return err
	} else {
		scratchpad = sp
	}

	data := make([]byte, 0, 3)
	data = append(data, byte(high), byte(low))
	switch s.familyCode {
	case 0x22, 0x28:
		data = append(data, scratchpad[4])
	}

	if err := s.writeScratchpad(data); err != nil {
		return err
	}
	return nil
}

func (s *TemperatureSensor) GetResolution() byte {
	return s.resolution
}

func (s *TemperatureSensor) SetResolution(resolution byte) error {
	switch s.familyCode {
	case 0x10:
		if resolution == BSResolution9bits {
			s.resolution = BSResolution9bits
			s.precision = "9 bits"
		} else {
			s.resolution = BS18S20ResolutionExtended
			s.precision = "extended"
		}
		return nil
	case 0x22, 0x28:
		var scratchpad []byte
		var err error
		if scratchpad, err = s.readScratchpad(); err != nil {
			return err
		}
		s.resolution = resolution & 0b11
		data := make([]byte, 0, 3)
		data = append(data, scratchpad[2], scratchpad[3])
		data = append(data, (s.resolution<<5)|0b00011111)
		if err := s.writeScratchpad(data); err != nil {
			return err
		}
		s.tConv = time.Millisecond * (750 / (8 >> s.resolution))
		s.precision = fmt.Sprintf("%d bits", 9+s.resolution)
	}
	return nil
}

//
// CONVERT T [44h]
// This command initiates a single temperature conversion.
//
func (s *TemperatureSensor) convertT() error {
	if err := s.reset(); err != nil {
		return err
	}
	if err := s.bus.WriteByte(0x44); err != nil {
		return err
	}
	if err := s.wait(s.tConv); err != nil {
		return err
	}
	return nil
}

//
// READ POWER SUPPLY [B4h]
// The bus driver issues this command to determine if devices on the bus are using parasite power.
//
func (s *TemperatureSensor) inParasiticMode() (bool, error) {
	if err := s.reset(); err != nil {
		return false, err
	}
	if err := s.bus.WriteByte(0xb4); err != nil {
		return false, err
	}
	if pm, err := s.bus.ReadBit(); err != nil {
		return false, err
	} else {
		return pm == 0b0, nil
	}
}

//
// READ SCRATCHPAD [BEh]
// This command allows the bus driver to read the contents of the scratchpad.
//
func (s *TemperatureSensor) readScratchpad() ([]byte, error) {
	if err := s.reset(); err != nil {
		return nil, err
	}
	if err := s.bus.WriteByte(0xbe); err != nil {
		return nil, err
	}
	var data = make([]byte, 9)
	if _, err := s.bus.ReadBytes(data); err != nil {
		return nil, err
	}
	scratchpad := data[0:8]
	crc := data[8]
	if crc8(scratchpad) != crc {
		return nil, errors.New("scratchpad crc error")
	}
	return scratchpad, nil
}

//
// WRITE SCRATCHPAD [4Eh]
// This command allows the master to write data to the device's scratchpad.
// All bytes MUST be written before the master issues a reset.
//
func (s *TemperatureSensor) writeScratchpad(data []byte) error {
	if err := s.reset(); err != nil {
		return err
	}
	if err := s.bus.WriteByte(0x4e); err != nil {
		return err
	}
	if _, err := s.bus.WriteBytes(data); err != nil {
		return err
	}
	return nil
}

//
// COPY SCRATCHPAD [48h]
// This command copies the contents of the scratchpad to EEPROM.
//
func (s *TemperatureSensor) copyScratchpad() error {
	if err := s.reset(); err != nil {
		return err
	}
	if err := s.bus.WriteByte(0x48); err != nil {
		return err
	}
	if err := s.wait(s.tRW); err != nil {
		return err
	}
	return nil
}

//
// RECALL EE [B8h]
// This command recalls values from EEPROM and places the data in the scratchpad memory.
//
func (s *TemperatureSensor) recallScratchpad() error {
	if s.parasiticMode {
		return nil
	}
	if err := s.reset(); err != nil {
		return err
	}
	if err := s.bus.WriteByte(0xb8); err != nil {
		return err
	}
	if err := s.wait(s.tConv); err != nil {
		return err
	}
	return nil
}

//
// Send reset pulse, wait for presence and then select the device.
//
func (s *TemperatureSensor) reset() error {
	if s.singleMode {
		return s.skipROM()
	} else {
		return s.matchROM(s.rom)
	}
}

//
// Wait for specified time in parasitic mode or until operation is finished in external power mode.
//
func (s *TemperatureSensor) wait(duration time.Duration) error {
	if s.parasiticMode {
		time.Sleep(duration)
	} else {
		startedAt := time.Now()
		for {
			if b, err := s.bus.ReadBit(); err != nil {
				return err
			} else {
				if b != 0b0 {
					break
				}
			}
			if time.Since(startedAt) > duration {
				break
			}
		}
	}
	return nil
}

//
// Read temperature value from the scratchpad
// Returns temperature * 10000 ºC
//
func (s *TemperatureSensor) calcTemperature(scratchpad []byte) int {
	var t int16
	_ = binary.Read(bytes.NewReader(scratchpad), binary.LittleEndian, &t)

	var temp int
	switch s.familyCode {
	case 0x10:
		temp = int(t) * 5000
		if s.resolution > BSResolution9bits {
			countRemain := int(scratchpad[6])
			countPerC := int(scratchpad[7])
			temp = temp - 2500 + 10000*(countPerC-countRemain)/countPerC
		}
	case 0x22, 0x28:
		temp = int(t) * 10000 / 16
	}
	return temp
}
