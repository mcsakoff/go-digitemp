package digitemp

func crc8(data []byte) byte {
	var crc byte = 0x00
	for _, b := range data {
		for i := 0; i < 8; i++ {
			mix := (crc ^ b) & 0x01
			crc >>= 1
			if mix > 0 {
				crc ^= 0x8c
			}
			b >>= 1
		}
	}
	return crc
}
