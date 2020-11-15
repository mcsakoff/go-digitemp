package digitemp

import "testing"

func TestNewROMFromBytes(t *testing.T) {
	bytes := []byte{0x28, 0x25, 0xea, 0x52, 0x05, 0x10, 0xf3, 0xce}
	str := "2825EA520510F3CE"
	rom := NewROMFromBytes(bytes)
	if rom.String() != str {
		t.Errorf("%s != %s", rom.String(), str)
	}
}

func TestNewROMFromString(t *testing.T) {
	bytes := []byte{0x28, 0x25, 0xea, 0x52, 0x05, 0x10, 0xf3, 0xce}
	str := "2825EA520510F3CE"
	if rom, err := NewROMFromString(str); err != nil {
		t.Error(err)
	} else if rom.String() != str {
		t.Errorf("%v != %v", rom.Code, bytes)
	}
}

func TestROM_toBits(t *testing.T) {
	str := "2825EA520510F3CE"
	rom, err := NewROMFromString(str)
	if err != nil {
		t.Fatal(err)
	}
	newRom := newRomFromBits(rom.toBits())
	if newRom.String() != str {
		t.Errorf("%s != %s", newRom.String(), str)
	}
}
