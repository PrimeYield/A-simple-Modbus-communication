package main

import (
	"fmt"
	"time"

	"github.com/goburrow/modbus"
)

func main() {
<<<<<<< HEAD
	handler := modbus.NewRTUClientHandler("COM4")
=======
	handler := modbus.NewRTUClientHandler("COM9")
>>>>>>> 3f2b2db0f47a77fe1b2950d209493d9b6f9daf8a
	handler.BaudRate = 9600
	handler.DataBits = 8
	handler.Parity = "N"
	handler.StopBits = 1
	// handler.SlaveId = 0
<<<<<<< HEAD
	handler.Timeout = 1 * time.Second
=======
	handler.Timeout = 5 * time.Second
>>>>>>> 3f2b2db0f47a77fe1b2950d209493d9b6f9daf8a

	for i := 0; i < 256; i++ {
		handler.SlaveId = byte(i)
		err := handler.Connect()
		if err != nil {
			fmt.Printf("not %d  ", i)
<<<<<<< HEAD
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
=======
			continue
		}
		fmt.Printf("your slaveID is %d", i)
		return
>>>>>>> 3f2b2db0f47a77fe1b2950d209493d9b6f9daf8a
	}
}
