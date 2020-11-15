package main

import (
	"github.com/mcsakoff/go-digitemp"
	"log"
)

func main() {
	uart, err := digitemp.NewUartAdapter("/dev/cu.usbserial-1410")
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		_ = uart.Close()
	}()

	log.Println("Searching ROMs")
	if roms, err := digitemp.NewAddressableDevice(uart).GetConnectedROMs(); err != nil {
		log.Fatal(err)
	} else {
		for n, rom := range roms {
			log.Printf("%d: %s\n", n, rom)
		}
	}
}
