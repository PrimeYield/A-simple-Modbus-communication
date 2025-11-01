package main

import (
	"fmt"
	"time"

	"github.com/goburrow/modbus"
)

func main() {
	handler := modbus.NewRTUClientHandler("COM4")
	handler.BaudRate = 9600
	handler.DataBits = 8
	handler.Parity = "N"
	handler.StopBits = 1
	// handler.SlaveId = 0
	handler.Timeout = 1 * time.Second

	for i := 0; i < 256; i++ {
		handler.SlaveId = byte(i)
		err := handler.Connect()
		if err != nil {
			fmt.Printf("not %d  ", i)
			handler.Close()
			continue
		}
		client := modbus.NewClient(handler)
		results, err := client.ReadHoldingRegisters(0, 3)
		if err != nil {
			fmt.Printf("not %d  ", i)
			handler.Close()
			continue
		} else {
			fmt.Println("slave ID is ", i)
			fmt.Println(results)
			return
		}
		// fmt.Println("slave ID is ", i)
		// fmt.Println(results)
	}
}
