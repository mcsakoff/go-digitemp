package main

import (
	"context"
	"fmt"
	"github.com/mcsakoff/go-digitemp"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

var app, stop = context.WithCancel(context.Background())

func main() {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigs
		log.Println("app: Got signal:", sig)
		stop()
	}()

	log.Println("Creating adapter")
	uart, err := digitemp.NewUartAdapter("/dev/cu.usbserial-1410")
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		_ = uart.Close()
	}()

	device := digitemp.NewAddressableDevice(uart)
	sensors := make([]*digitemp.TemperatureSensor, 0)

	log.Println("Searching ROMs")
	if roms, err := device.GetConnectedROMs(); err != nil {
		log.Fatal(err)
	} else {
		for n, rom := range roms {
			log.Printf("%d: %s\n", n, rom)
			if sensor, err := digitemp.NewTemperatureSensor(uart, rom, true); err != nil {
				log.Fatal(err)
			} else {
				sensors = append(sensors, sensor)
			}
		}
	}

	for _, sensor := range sensors {
		log.Printf("====================================================\n")
		log.Printf("    Device: %s", sensor.GetName())
		log.Printf("       ROM: %s", sensor.GetROM())
		log.Printf(" Parasitic: %t", sensor.IsParasiticMode())
		if err := sensor.SetResolution(digitemp.BS18B20Resolution12bits); err != nil {
			log.Println("failed to set resolution")
		}
		log.Printf("Resolution: %s", sensor.GetPrecision())
	}
	log.Printf("====================================================\n")

	go func() {
		measurements := make([]string, len(sensors))
		for {
			for n, sensor := range sensors {
				if tc, err := sensor.GetTemperatureFloat(); err != nil {
					measurements[n] = "error"
				} else {
					measurements[n] = fmt.Sprintf("%.02f", tc)
				}
			}
			log.Println(strings.Join(measurements, "   "))
			time.Sleep(3 * time.Second)
		}
	}()
	<-app.Done()
}
