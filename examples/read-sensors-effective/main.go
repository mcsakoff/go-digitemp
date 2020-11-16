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

	sensors := make([]*digitemp.TemperatureSensor, 0)

	log.Println("Searching ROMs")
	if roms, err := uart.GetConnectedROMs(); err != nil {
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
		if err := sensor.SetResolution(digitemp.Resolution12bits); err != nil {
			log.Println("failed to set resolution")
		}
		log.Printf("Resolution: %s", sensor.GetPrecision())
	}
	log.Printf("====================================================\n")

	// Instead of calling GetTemperature() for each sensor we call uart.MeasureTemperatureAll() once
	// and then do sensor.ReadTemperature() for each sensor.
	go func() {
		measurements := make([]string, len(sensors))
		for {
			if err := uart.MeasureTemperatureAll(); err != nil {
				log.Print(err)
				continue
			}
			for n, sensor := range sensors {
				if tc, err := sensor.ReadTemperatureFloat(); err != nil {
					measurements[n] = "error"
				} else {
					measurements[n] = fmt.Sprintf("%.02fºC", tc)
				}
			}
			log.Println(strings.Join(measurements, "   "))
			time.Sleep(3 * time.Second)
		}
	}()
	<-app.Done()
}
