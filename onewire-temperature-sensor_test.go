package digitemp

import "testing"

type calcTemperatureTestcase struct {
	scratchpad  []byte
	temperature int
}

func TestTemperatureSensor_DS18B20_calcTemperature(t *testing.T) {
	var sensor = TemperatureSensor{
		familyCode: 0x28, // DS18B20, DS1822
	}
	var testcases = []calcTemperatureTestcase{
		{[]byte{0xd0, 0x07}, 1250000}, // 125.0
		{[]byte{0x50, 0x05}, 850000},  //  85.0
		{[]byte{0x91, 0x01}, 250625},  //  25.0625
		{[]byte{0xa2, 0x00}, 101250},  //  10.125
		{[]byte{0x08, 0x00}, 5000},    //  50.0
		{[]byte{0x00, 0x00}, 0},       //   0.0
		{[]byte{0xf8, 0xff}, -5000},   // -50.0
		{[]byte{0x5e, 0xff}, -101250}, // -10.125
		{[]byte{0x6f, 0xfe}, -250625}, // -25.0625
		{[]byte{0x90, 0xfc}, -550000}, // -55.0
	}
	var scrpad = [8]byte{0x00, 0x00, 0x00, 0x00, 0x00, 0xff, 0x00, 0x10}

	for n, tc := range testcases {
		scrpad[0] = tc.scratchpad[0]
		scrpad[1] = tc.scratchpad[1]
		if temp := sensor.calcTemperature(scrpad[:]); temp != tc.temperature {
			t.Errorf("(%d, got: %d, expected: %d)", n, temp, tc.temperature)
		}
	}
}

func TestTemperatureSensor_DS18S20_calcTemperature(t *testing.T) {
	var sensor = TemperatureSensor{
		familyCode: 0x10, // DS18S20
		resolution: BSResolution9bits,
	}
	var testcases = []calcTemperatureTestcase{
		{[]byte{0xaa, 0x00}, 850000},  //  85.0
		{[]byte{0x32, 0x00}, 250000},  //  25.0
		{[]byte{0x01, 0x00}, 5000},    //   0.5
		{[]byte{0x00, 0x00}, 0},       //   0.0
		{[]byte{0xff, 0xff}, -5000},   //  -0.5
		{[]byte{0xce, 0xff}, -250000}, // -25.0
		{[]byte{0x92, 0xff}, -550000}, // -55.0
	}
	var scrpad = [8]byte{0x00, 0x00, 0x00, 0x00, 0xff, 0xff, 0x0C, 0x10}

	for n, tc := range testcases {
		scrpad[0] = tc.scratchpad[0]
		scrpad[1] = tc.scratchpad[1]
		if temp := sensor.calcTemperature(scrpad[:]); temp != tc.temperature {
			t.Errorf("(%d, got: %d, expected: %d)", n, temp, tc.temperature)
		}
	}
}
