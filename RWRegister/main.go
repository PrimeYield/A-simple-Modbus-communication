package main

import (
	"log"
	"time"

	"github.com/goburrow/modbus"
)

func main() {
	// Modbus RTU/ASCII
	handler := modbus.NewRTUClientHandler("COM5")
	// handler := modbus.NewRTUClientHandler("\\\\.\\COM13")
	handler.BaudRate = 9600
	handler.DataBits = 8
	handler.Parity = "N"
	handler.StopBits = 1
	handler.SlaveId = 1
	handler.Timeout = 5 * time.Second

	// weight := 0

	// for i := 0; i < 100; i++ {
	err := handler.Connect()
	if err != nil {
		log.Printf("Connect error: %v", err)
		return
	} else {
		log.Printf("Connect success")
	}
	defer handler.Close()

	client := modbus.NewClient(handler)
	results, err := client.ReadHoldingRegisters(0, 3)
	// results, err := client.WriteSingleRegister(81, 10)
	// results, err := client.WriteSingleRegister(50, 1)
	if err != nil {
		log.Printf("ReadHoldingRegisters error: %v", err)
		return
	}
	log.Println(results)

	// weight = 800
	// results_int, err := strconv.Atoi(string(results))
	// if err != nil {
	// 	log.Printf("trans to int fail %v ... ", err)
	// 	return
	// }
	// results_6ad := results_int * weight
	// log.Println(results_6ad)
	// os.WriteFile()
	// filePath := "D:\\huanglin\\testGolangOS\\hikari.txt"
	// flags := os.O_WRONLY | os.O_CREATE | os.O_APPEND
	// permission := os.FileMode(0644)
	// // err = os.WriteFile(filePath, results, permission)
	// file, err := os.OpenFile(filePath, flags, permission)
	// if err != nil {
	// 	log.Printf("WriteFile error: %v", err)
	// 	return
	// }
	// defer file.Close()
	// _, err = file.WriteString(fmt.Sprint(time.Now(), results, "\n"))
	// log.Printf("write success")
	// time.Sleep(60 * time.Second)
	// file.Close()
	// handler.Close()
	// if i == 99 {
	// 	break
	// }
	// time.Sleep(60 * time.Second)
	// }
}
