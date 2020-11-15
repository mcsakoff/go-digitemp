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

	// Works when only one sensor connected
	sensor, err := digitemp.NewTemperatureSensor(uart, nil, true)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("     Device: %s", sensor.GetName())
	log.Printf("        ROM: %s", sensor.GetROM())
	log.Printf("  Parasitic: %t", sensor.IsParasiticMode())
	log.Printf(" Resolution: %s", sensor.GetPrecision())

	temp, err := sensor.GetTemperatureFloat()
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("Temperature: %.02f\n", temp)
}
