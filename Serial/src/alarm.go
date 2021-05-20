package main

import (
	"fmt"
	"sync"
	"time"
	"bytes"
    "encoding/binary"
)

var recordMux sync.Mutex

func BytesToInt(b []byte) int {
    bytesBuffer := bytes.NewBuffer(b)
    var x int32
    binary.Read(bytesBuffer, binary.BigEndian, &x)
    return int(x)
}

func CheckAlarm(b []byte, timeStamp int64) {
	defer endingMux.Done()
	var sum byte = 0
	for i := 3; i < len(b); i++ {
		sum += b[i]
	}
	
	serialNumber_byte:=[]byte{0,0,b[1],b[2]}
	serialNumber:=BytesToInt(serialNumber_byte) //读取流水号
	
	/* if sum != b[0] {
		recordMux.Lock()
		fmt.Printf("%d: Wrong data!\n", timeStamp)
		recordMux.Unlock()
		return
	}*/
	if len(b) < 20 {
		recordMux.Lock()
		fmt.Printf("%d: Length error!\n", timeStamp)
		recordMux.Unlock()
		return
	}
	if b[5] == 0x65 && b[6] == 0x36 {
		recordMux.Lock()
		switch b[12] {
		case 0x01:
			fmt.Printf("%d,%d: Alarm type:1,%d\n", serialNumber,timeStamp,b[13]) //疲劳驾驶，记录疲劳程度
		case 0x02,0x03,0x04,0x05:
			fmt.Printf("%d,%d: Alarm type:%d\n", serialNumber,timeStamp, b[12])
		default:
			fmt.Printf("%d,%d: Unknown alarm type.\n", serialNumber,timeStamp)
		}
		recordMux.Unlock()
	}
}

func test_CheckAlarm() {
	t := time.Now().Unix()
	var response []byte
	response = []byte{0x36, 0, 0x0b, 0x00, 0x01, 0x65, 0x36, 0, 0, 0, 0x0a, 0, 5, 0, 0, 0xc0, 0xcb, 0, 0, 0, 0, 0, 0}
	go CheckAlarm(response, t)
	response = []byte{0x35, 0, 0x0b, 0x00, 0x01, 0x65, 0x36, 0, 0, 0, 0x0a, 0, 4, 0, 0, 0xc0, 0xcb, 0, 0, 0, 0, 0, 0}
	go CheckAlarm(response, t)
	response = []byte{0x31, 0, 0x0b, 0x00, 0x01, 0x65, 0x36, 0, 0, 0, 0x0a, 0, 0, 0, 0, 0xc0, 0xcb, 0, 0, 0, 0, 0, 0}
	go CheckAlarm(response, t)
	time.Sleep(1000)
}
