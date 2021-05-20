package main

import (
	_ "bytes"
	_"fmt"
	"github.com/tarm/goserial"
	"io"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

type JSerror struct {
	num int
}

func (err JSerror) Error() string {
	switch err.num {
	case 0:
		return "Frame signal."
	case 1:
		return "Escaping failed!"
	default:
		return "Unknown error!"
	}
}

type JSStandard struct {
	r io.ReadWriter
}

func (s JSStandard) Read(b []byte) (n int, err error) {
	c := make([]byte, 1)
	escape := false
	for i := 0; i < len(b); {
		m, err_r := s.r.Read(c)
		if err_r != nil {
			return i, err_r
		}
		if m == 0 {
			return i, io.EOF
		}
		if escape {
			switch c[0] {
			case 0x01:
				b[i] = 0x7d
			case 0x02:
				b[i] = 0x7e
			default:
				return i, JSerror{1}
			}
			escape = false
			i++
		} else {
			switch c[0] {
			case 0x7d:
				escape = true
			case 0x7e:
				b[i] = 0x7e
				return i + 1, JSerror{0} //避免被当成无内容
			default:
				b[i] = c[0]
				i++
			}
		}
	}
	return len(b), nil
}

var endingMux sync.WaitGroup
var stopNewInfo sync.Mutex

func (s JSStandard) Write(b []byte) (n int, err error) {
	var afterTrans []byte
	afterTrans = append(afterTrans, 0x7e)
	for _, c := range b {
		switch c {
		case 0x7d:
			afterTrans = append(afterTrans, 0x7d, 0x01)
		case 0x7e:
			afterTrans = append(afterTrans, 0x7d, 0x02)
		default:
			afterTrans = append(afterTrans, c)
		}
	}
	afterTrans = append(afterTrans, 0x7e)
	_, err_r := s.r.Write(afterTrans)
	return len(b), err_r
}

func main() {
	c := &serial.Config{Name: "/dev/ttyUSB0", Baud: 115200}

	ttyRW, err := serial.OpenPort(c)
	if err != nil {
		log.Fatal(err)
	}

	//处理系统的kill命令
	endingSig := make(chan os.Signal)
	signal.Notify(endingSig, syscall.SIGHUP, syscall.SIGINT, syscall.SIGQUIT, syscall.SIGTERM)
	go func() {
		for _ = range endingSig {
			stopNewInfo.Lock() //停止新记录的接收
			endingMux.Wait()   //等待待记录的都记录完
			log.Println("Ending by kill.")
			ttyRW.Close()
			os.Exit(0)
		}
	}()

	query_existence := []byte{0x7e, 0x95, 0x00, 0x00, 0x00, 0x01, 0x65, 0x2f, 0x7e}
	//query_parameters := []byte{0x7e, 0x9a, 0x00, 0x00, 0x00, 0x01, 0x65, 0x34, 0x7e}
	answer_alarm := make([]byte, 7)
	//query_alarm:=[]byte{0x7e,0x9c,0x00,0x00,0x00,0x01,0x65,0x36,0x7e}*/

	/*bytedata := []byte{5, 5, 5, 0x7e, 0x36, 0, 0x0b, 0x00, 0x01, 0x65, 0x36, 0, 0, 0, 0x0a, 0, 5, 0, 0, 0xc0, 0xcb, 0, 0, 0, 0, 0, 0, 0x7e,
		0x7e, 0x35, 0, 0x0b, 0x00, 0x01, 0x65, 0x36, 0, 0, 0, 0x0a, 0, 4, 0, 0, 0xc0, 0xcb, 0, 0, 0, 0, 0, 0, 0x7e,
		0x7e, 0x31, 0, 0x0b, 0x00, 0x01, 0x65, 0x36, 0, 0, 0, 0x0a, 0, 0, 0, 0, 0xc0, 0xcb, 0, 0, 0, 0, 0, 0, 0x7e, 4, 4, 4}
	ttyRws := bytes.NewReader(bytedata)*/

	jstransfer := JSStandard{ttyRW} //转义层

	_, err = ttyRW.Write(query_existence) //通过询问存在唤醒
	if err != nil {
		log.Fatal(err)
	}
	time.Sleep(50 * time.Millisecond)
	buf := make([]byte, 128)

	for {
		//重置
		newframe := false
		var fromArcSoft []byte
		wating := false
		var timeStamp int64
		for {
			n, err := jstransfer.Read(buf)
			if n == 0 {
				if newframe && !wating {
					//如果当前帧还未读完，再次尝试
					wating = true
					time.Sleep(100 * time.Millisecond)
					continue
				}
				break //暂时无消息可读，休息
			}

			frameSignal := false
			if err != nil {
				if err.Error() != "Frame signal." {
					log.Fatal(err)
					continue
				}
				frameSignal = true
			}

			for i := 0; i < n; i++ {
				fromArcSoft = append(fromArcSoft, buf[i])
			} //先加到待处理序列
			if frameSignal {
				if len(fromArcSoft) == 1 {
					//遇到7e7e的情况，会在后一个7e再次刷新
					stopNewInfo.Lock()
					stopNewInfo.Unlock()
					newframe = true
					timeStamp = time.Now().Unix() //先记下时间戳
					goto BufferClear
				}
				if newframe {
					newframe = false
					if fromArcSoft[6] != 0x36 {
						goto BufferClear
					}
					var tmp_sum byte = 0
					for i := 1; i < len(answer_alarm); i++ {
						answer_alarm[i] = fromArcSoft[i]
					}
					for i := 3; i < len(answer_alarm); i++ {
						tmp_sum += answer_alarm[i]
					}
					answer_alarm[0] = tmp_sum //根据协议，先进行校验计算，再转义
					jstransfer.Write(answer_alarm)
					endingMux.Add(1)
					go CheckAlarm(fromArcSoft,timeStamp)
				}
				BufferClear: fromArcSoft = []byte{}
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
}
