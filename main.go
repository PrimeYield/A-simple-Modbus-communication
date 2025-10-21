package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/goburrow/modbus"
)

func main() {
	// Modbus RTU/ASCII
	handler := modbus.NewRTUClientHandler("\\\\.\\COM11")
	handler.BaudRate = 19200
	handler.DataBits = 8
	handler.Parity = "E"
	handler.StopBits = 1
	handler.SlaveId = 1
	handler.Timeout = 5 * time.Second

	// for i := 0; i < 100; i++ {
	err := handler.Connect()
	if err != nil {
		log.Printf("Connect error: %v", err)
		return
	}
	defer handler.Close()

	client := modbus.NewClient(handler)
	results, err := client.ReadHoldingRegisters(40, 1)
	if err != nil {
		log.Printf("ReadHoldingRegisters error: %v", err)
	}
	log.Println(results)
	// os.WriteFile()
	filePath := "D:\\huanglin\\testGolangOS\\hikari.txt"
	flags := os.O_WRONLY | os.O_CREATE | os.O_APPEND
	permission := os.FileMode(0644)
	// err = os.WriteFile(filePath, results, permission)
	file, err := os.OpenFile(filePath, flags, permission)
	if err != nil {
		log.Printf("WriteFile error: %v", err)
		return
	}
	defer file.Close()
	_, err = file.WriteString(fmt.Sprint(time.Now(), results, "\n"))
	log.Printf("write success")
	// time.Sleep(60 * time.Second)
	// file.Close()
	// handler.Close()
	// if i == 99 {
	// 	break
	// }
	time.Sleep(60 * time.Second)
	// }
}
