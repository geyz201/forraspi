package main

import (
	"flag"
	"fmt"
	"log"
	_"strings"
	"time"
	"os"
	"os/signal"
	"syscall"
	"github.com/paypal/gatt"
	"github.com/paypal/gatt/examples/option"
)

func onStateChanged(d gatt.Device, s gatt.State) {
	log.Println("State:", s)
	switch s {
	case gatt.StatePoweredOn:
		log.Println("Scanning...")
		d.Scan([]gatt.UUID{}, false)
		return
	default:
		d.StopScanning()
	}
}

func onPeriphDiscovered(p gatt.Peripheral, a *gatt.Advertisement, rssi int) {
	if len(a.Services)!=2 {
		return
	}
	ads0:=a.Services[0]
	ads1:=a.Services[1]
	if ads0.Equal(gatt.UUID16(0xffff)) && ads1.Equal(gatt.UUID16(0x180d)) {
		p.Device().StopScanning()
		p.Device().Connect(p)
	} else {
		return
	}
}

func onPeriphConnected(p gatt.Peripheral, err error) {
	log.Printf("Connected to %s, ID: %s\n",p.Name(),p.ID())
	//defer p.Device().CancelConnection(p)

	if err := p.SetMTU(500); err != nil {
		log.Printf("Failed to set MTU, err: %s\n", err)
	}
	
	var char_command *gatt.Characteristic=nil
	var char_response *gatt.Characteristic=nil

	sid:=gatt.MustParseUUID("00001523-1212-efde-1523-785feabcd123")
	ss, err := p.DiscoverServices(nil)
	if err != nil {
		log.Printf("Failed to discover services, err: %s\n", err)
		return
	}
	for _, s := range ss {
		if !sid.Equal(s.UUID()) {
			continue
		}
		
		//搜寻指定Characteristic
		cidCommand := gatt.MustParseUUID("00001027-1212-efde-1523-785feabcd123")
		cidResponse := gatt.MustParseUUID("00001011-1212-efde-1523-785feabcd123")

		cids := []gatt.UUID{cidCommand, cidResponse}
		cs, err := p.DiscoverCharacteristics(cids, s)
		if err != nil {
			log.Printf("Failure of discovering characteristics, err: %s\n", err)
			//continue
		}

		for _, c := range cs {
			
			switch {
			case c.UUID().Equal(cidCommand):
				char_command = c
			case c.UUID().Equal(cidResponse):
				char_response = c
			}

			// Discovery descriptors
			_, err := p.DiscoverDescriptors(nil, c)
			if err != nil {
				log.Printf("Failed to discover descriptors, err: %s\n", err)
				continue
			}
		}
		fmt.Println()
	}
	f := func(c *gatt.Characteristic, b []byte, err error) {
		if (b[0]==0xaa) {
			fmt.Println(b[1:])
		} else {
			fmt.Printf("%s", b)
		}
	}
	if err := p.SetNotifyValue(char_response, f); err != nil {
		log.Fatalf("Failed to subscribe characteristic, err: %s\n", err)
	}
	go ASRecord(p,char_command)
	log.Println("All is ok!")
}

func onPeriphDisconnected(p gatt.Peripheral, err error) {
	log.Println("Disconnected")
}

func ASRecord(p gatt.Peripheral,char *gatt.Characteristic) {
	//处理系统的kill命令
	endingSig := make(chan os.Signal)
	signal.Notify(endingSig, syscall.SIGHUP, syscall.SIGINT, syscall.SIGQUIT, syscall.SIGTERM)
	go func() {
		for _ = range endingSig {
			p.WriteCharacteristic(char,[]byte("stop\n"),true)
			log.Println("Ending by kill.")
			time.Sleep(500*time.Millisecond)
			p.Device().CancelConnection(p)
			os.Exit(0)
		}
	}()

	p.WriteCharacteristic(char,[]byte("reset\n"),true)
	p.WriteCharacteristic(char,[]byte("get_device_info\n"),true)
	p.WriteCharacteristic(char,[]byte("get_format ppg 5*\n"),true)
	p.WriteCharacteristic(char,[]byte("read ppg 5*\n"),true) 
}

func main() {
	flag.Parse()

	d, err := gatt.NewDevice(option.DefaultClientOptions...)
	if err != nil {
		log.Fatalf("Failed to open device, err: %s\n", err)
		return
	}

	// Register handlers.
	d.Handle(
		gatt.PeripheralDiscovered(onPeriphDiscovered),
		gatt.PeripheralConnected(onPeriphConnected),
		gatt.PeripheralDisconnected(onPeriphDisconnected),
	)

	d.Init(onStateChanged)
	select{}
}
