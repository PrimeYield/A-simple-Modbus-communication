package main

import (
	"fmt"
	"time"

	"github.com/goburrow/modbus"
)

func main() {
	handler := modbus.NewRTUClientHandler("COM9")
	handler.BaudRate = 9600
	handler.DataBits = 8
	handler.Parity = "N"
	handler.StopBits = 1
	// handler.SlaveId = 0
	handler.Timeout = 5 * time.Second

	for i := 0; i < 256; i++ {
		handler.SlaveId = byte(i)
		err := handler.Connect()
		if err != nil {
			fmt.Printf("not %d  ", i)
			continue
		}
		fmt.Printf("your slaveID is %d", i)
		return
	}
}
