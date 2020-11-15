package digitemp

import (
	"fmt"
	"strconv"
	"strings"
)

type ROM struct {
	Code [8]byte
}

func NewROMFromBytes(code []byte) *ROM {
	r := new(ROM)
	copy(r.Code[0:8], code)
	return r
}

func NewROMFromString(code string) (*ROM, error) {
	if len(code) != 16 {
		return nil, fmt.Errorf("wrong rom code length")
	}
	r := new(ROM)
	for i := 0; i < 8; i += 1 {
		if b, err := strconv.ParseInt(code[i*2:i*2+2], 16, 16); err != nil {
			return nil, err
		} else {
			r.Code[i] = byte(b)
		}
	}
	return r, nil
}

func (r *ROM) String() string {
	var bytes = make([]string, 8)
	for _, b := range r.Code {
		bytes = append(bytes, fmt.Sprintf("%02X", b))
	}
	return strings.Join(bytes, "")
}

func (r *ROM) IsValid() bool {
	return crc8(r.Code[0:7]) == r.Code[7]
}

func newRomFromBits(bits []byte) *ROM {
	r := new(ROM)
	for n := 0; n < 8; n++ {
		r.Code[n] = 0x00
	}
	for n, bit := range bits {
		if n >= 64 {
			break
		}
		if bit > 0b0 {
			r.Code[n/8] |= byte(0x01) << (n % 8)
		}
	}
	return r
}

func (r *ROM) toBits() []byte {
	bits := make([]byte, 0, 64)
	for _, b := range r.Code {
		for m := 0; m < 8; m++ {
			bits = append(bits, b%2)
			b >>= 1
		}
	}
	return bits
}
