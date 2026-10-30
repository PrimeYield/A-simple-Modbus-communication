package main

import (
	"bufio"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"go.bug.st/serial"
)

const (
	serialPort = "COM8"
	baudRate   = 9600
	slaveID    = byte(1)
	logFile    = "modbus.log"
)

// RegisterStore 是 Terminal 與 PLC 共用的資料層。
type RegisterStore struct {
	mu     sync.RWMutex
	values map[uint16]uint16
}

func NewRegisterStore() *RegisterStore {
	return &RegisterStore{values: make(map[uint16]uint16)}
}

func (s *RegisterStore) Set(addr, value uint16) {
	s.mu.Lock()
	s.values[addr] = value
	s.mu.Unlock()
}

func (s *RegisterStore) Get(addr uint16) uint16 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	// 未設定的 Register 預設回傳 0。
	return s.values[addr]
}

type RegisterItem struct {
	Addr  uint16
	Value uint16
}

func (s *RegisterStore) List() []RegisterItem {
	s.mu.RLock()
	defer s.mu.RUnlock()

	items := make([]RegisterItem, 0, len(s.values))
	for addr, value := range s.values {
		items = append(items, RegisterItem{Addr: addr, Value: value})
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].Addr < items[j].Addr
	})
	return items
}

// 支援 FATEK 參考位址 400001~465536，以及 PDU 位址 0~65535。
func parseAddress(input string) (uint16, error) {
	n, err := strconv.ParseUint(input, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("無效的位址：%s", input)
	}

	if n >= 400001 && n <= 465536 {
		n -= 400001
	}
	if n > 65535 {
		return 0, fmt.Errorf("位址超出範圍：%s", input)
	}
	return uint16(n), nil
}

// Terminal 輸入十進位數值，範圍 0~65535。
func parseValue(input string) (uint16, error) {
	n, err := strconv.ParseUint(input, 10, 16)
	if err != nil {
		return 0, fmt.Errorf("數值必須是 0~65535 的十進位整數：%s", input)
	}
	return uint16(n), nil
}

func fatekAddress(addr uint16) uint32 {
	return uint32(addr) + 400001
}

func crc16(data []byte) uint16 {
	crc := uint16(0xFFFF)

	for _, b := range data {
		crc ^= uint16(b)
		for i := 0; i < 8; i++ {
			if crc&1 != 0 {
				crc = (crc >> 1) ^ 0xA001
			} else {
				crc >>= 1
			}
		}
	}
	return crc
}

func validCRC(frame []byte) bool {
	if len(frame) < 4 {
		return false
	}
	got := binary.LittleEndian.Uint16(frame[len(frame)-2:])
	return crc16(frame[:len(frame)-2]) == got
}

func appendCRC(frame []byte) []byte {
	crc := crc16(frame)
	return append(frame, byte(crc), byte(crc>>8))
}

func exceptionResponse(function, code byte) []byte {
	return appendCRC([]byte{slaveID, function | 0x80, code})
}

// 所有 Modbus Register 值均以 uint16 保存，並以十進位顯示。
// 封包中的兩個資料位元組必須先用 BigEndian 合併成 16-bit 整數。
func handleRequest(frame []byte, store *RegisterStore, logger *log.Logger) []byte {
	if len(frame) < 4 {
		logger.Printf("ERROR: 封包太短：% X", frame)
		return nil
	}
	if !validCRC(frame) {
		logger.Printf("ERROR: CRC 錯誤：RX=% X", frame)
		return nil
	}
	if frame[0] != slaveID {
		logger.Printf("INFO: 忽略 Slave ID=%d：RX=% X", frame[0], frame)
		return nil
	}

	function := frame[1]

	switch function {
	case 0x03: // Read Holding Registers
		if len(frame) != 8 {
			return exceptionResponse(function, 0x03)
		}

		start := binary.BigEndian.Uint16(frame[2:4])
		quantity := binary.BigEndian.Uint16(frame[4:6])

		if quantity == 0 || quantity > 125 {
			return exceptionResponse(function, 0x03)
		}
		if uint32(start)+uint32(quantity) > 65536 {
			return exceptionResponse(function, 0x02)
		}

		response := []byte{slaveID, function, byte(quantity * 2)}
		for i := uint16(0); i < quantity; i++ {
			addr := start + i
			value := store.Get(addr)

			response = append(response, byte(value>>8), byte(value))
			logger.Printf("READ PDU=%d FATEK=%d value=%d (0x%04X)",
				addr, fatekAddress(addr), value, value)
		}
		return appendCRC(response)

	case 0x06: // Write Single Register
		if len(frame) != 8 {
			return exceptionResponse(function, 0x03)
		}

		addr := binary.BigEndian.Uint16(frame[2:4])
		value := binary.BigEndian.Uint16(frame[4:6])
		store.Set(addr, value)

		logger.Printf(
			"WRITE PDU=%d FATEK=%d raw=%02X %02X decimal=%d hex=0x%04X",
			addr, fatekAddress(addr), frame[4], frame[5], value, value,
		)

		// 0x06 正常回應：回傳原始請求的前 6 bytes，再附 CRC。
		return appendCRC(append([]byte(nil), frame[:6]...))

	case 0x10: // Write Multiple Registers
		if len(frame) < 9 {
			return exceptionResponse(function, 0x03)
		}

		start := binary.BigEndian.Uint16(frame[2:4])
		quantity := binary.BigEndian.Uint16(frame[4:6])
		byteCount := int(frame[6])

		if quantity == 0 || quantity > 123 ||
			byteCount != int(quantity)*2 ||
			len(frame) != 9+byteCount {
			return exceptionResponse(function, 0x03)
		}
		if uint32(start)+uint32(quantity) > 65536 {
			return exceptionResponse(function, 0x02)
		}

		for i := uint16(0); i < quantity; i++ {
			offset := 7 + int(i)*2
			value := binary.BigEndian.Uint16(frame[offset : offset+2])
			addr := start + i

			store.Set(addr, value)
			logger.Printf("WRITE MULTIPLE PDU=%d FATEK=%d decimal=%d hex=0x%04X",
				addr, fatekAddress(addr), value, value)
		}

		response := []byte{
			slaveID, function,
			byte(start >> 8), byte(start),
			byte(quantity >> 8), byte(quantity),
		}
		return appendCRC(response)

	default:
		logger.Printf("ERROR: 不支援的功能碼 0x%02X", function)
		return exceptionResponse(function, 0x01)
	}
}

// 簡易封包接收：首位元組逾時 500ms，收到資料後以 5ms 間隔判斷封包結束。
// 若有 CRC 錯誤或拆包問題，需要再改用更嚴謹的 RTU 定界機制。
func readFrame(port serial.Port) ([]byte, error) {
	if err := port.SetReadTimeout(500 * time.Millisecond); err != nil {
		return nil, err
	}

	frame := make([]byte, 0, 64)
	one := make([]byte, 1)

	for {
		n, err := port.Read(one)
		if err != nil {
			if len(frame) > 0 {
				return frame, nil
			}
			if errors.Is(err, io.EOF) {
				return nil, nil
			}
			return nil, err
		}

		if n == 0 {
			if len(frame) == 0 {
				return nil, nil
			}
			return frame, nil
		}

		frame = append(frame, one[0])
		if len(frame) >= 256 {
			return frame, errors.New("RTU 封包過長")
		}

		if err := port.SetReadTimeout(5 * time.Millisecond); err != nil {
			return nil, err
		}
	}
}

func runSerial(
	port serial.Port,
	store *RegisterStore,
	logger *log.Logger,
	stop <-chan struct{},
	done chan<- struct{},
) {
	defer close(done)
	logger.Printf("Slave 啟動：COM=%s baud=%d 8N1 SlaveID=%d",
		serialPort, baudRate, slaveID)

	for {
		select {
		case <-stop:
			logger.Printf("背景通訊停止")
			return
		default:
		}

		frame, err := readFrame(port)
		if err != nil {
			select {
			case <-stop:
				return
			default:
			}
			logger.Printf("ERROR: 讀取序列埠失敗：%v", err)
			continue
		}
		if len(frame) == 0 {
			continue
		}

		logger.Printf("RX (%d bytes): % X", len(frame), frame)

		response := handleRequest(frame, store, logger)
		if len(response) == 0 {
			continue
		}

		if _, err := port.Write(response); err != nil {
			logger.Printf("ERROR: 寫入序列埠失敗：%v", err)
			continue
		}
		logger.Printf("TX (%d bytes): % X", len(response), response)
	}
}

func printHelp() {
	fmt.Println("指令：")
	fmt.Println("  set <位址> <數值>  設定 Register，例如 set 400001 1234")
	fmt.Println("  get <位址>         查詢 Register，例如 get 400001")
	fmt.Println("  list               列出已設定的 Register")
	fmt.Println("  help               顯示說明")
	fmt.Println("  exit               結束程式")
	fmt.Println()
	fmt.Println("位址：FATEK 400001~465536 或 PDU 0~65535")
	fmt.Println("數值：十進位整數 0~65535")
	fmt.Println("封包日誌：modbus.log")
}

func runConsole(store *RegisterStore, stop chan struct{}, port serial.Port, logger *log.Logger) {
	scanner := bufio.NewScanner(os.Stdin)
	printHelp()

	for {
		fmt.Print("> ")
		if !scanner.Scan() {
			if err := scanner.Err(); err != nil {
				fmt.Printf("Terminal 輸入錯誤：%v\n", err)
			}
			close(stop)
			_ = port.Close()
			return
		}

		fields := strings.Fields(scanner.Text())
		if len(fields) == 0 {
			continue
		}

		switch strings.ToLower(fields[0]) {
		case "set":
			if len(fields) != 3 {
				fmt.Println("用法：set <位址> <數值>")
				continue
			}

			addr, err := parseAddress(fields[1])
			if err != nil {
				fmt.Println(err)
				continue
			}
			value, err := parseValue(fields[2])
			if err != nil {
				fmt.Println(err)
				continue
			}

			store.Set(addr, value)
			fmt.Printf("已設定：FATEK %d (PDU %d) = %d (0x%04X)\n",
				fatekAddress(addr), addr, value, value)

		case "get":
			if len(fields) != 2 {
				fmt.Println("用法：get <位址>")
				continue
			}

			addr, err := parseAddress(fields[1])
			if err != nil {
				fmt.Println(err)
				continue
			}

			value := store.Get(addr)
			fmt.Printf("FATEK %d (PDU %d) = %d (0x%04X)\n",
				fatekAddress(addr), addr, value, value)

		case "list":
			items := store.List()
			if len(items) == 0 {
				fmt.Println("尚未明確設定任何 Register；未設定的位址讀取時回傳 0。")
				continue
			}
			for _, item := range items {
				fmt.Printf("FATEK %-6d PDU %-5d = %-5d (0x%04X)\n",
					fatekAddress(item.Addr), item.Addr, item.Value, item.Value)
			}

		case "help":
			printHelp()

		case "exit", "quit":
			fmt.Println("正在停止背景通訊...")
			close(stop)
			_ = port.Close()
			logger.Printf("使用者要求結束程式")
			return

		default:
			fmt.Println("未知指令，輸入 help 查看用法。")
		}
	}
}

func main() {
	logOutput, err := os.OpenFile(
		logFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644,
	)
	if err != nil {
		fmt.Printf("無法開啟日誌檔 %s：%v\n", logFile, err)
		os.Exit(1)
	}
	defer logOutput.Close()

	logger := log.New(logOutput, "", log.Ldate|log.Ltime|log.Lmicroseconds)

	mode := &serial.Mode{
		BaudRate: baudRate,
		DataBits: 8,
		Parity:   serial.NoParity,
		StopBits: serial.OneStopBit,
	}
	port, err := serial.Open(serialPort, mode)
	if err != nil {
		logger.Printf("ERROR: 無法開啟 %s：%v", serialPort, err)
		fmt.Printf("無法開啟 %s：%v\n詳細資訊請查看 %s\n",
			serialPort, err, logFile)
		os.Exit(1)
	}

	store := NewRegisterStore()
	stop := make(chan struct{})
	done := make(chan struct{})

	go runSerial(port, store, logger, stop, done)

	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-signalCh
		fmt.Println("\n收到停止訊號，正在關閉...")
		select {
		case <-stop:
		default:
			close(stop)
		}
		_ = port.Close()
	}()

	runConsole(store, stop, port, logger)
	<-done
	signal.Stop(signalCh)

	logger.Printf("程式結束")
	fmt.Printf("程式已結束，封包日誌：%s\n", logFile)
}
