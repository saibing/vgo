package proxy

import (
	"fmt"
	"time"
)

func logRequest(format string, a ...interface{}) {
	now := time.Now().Format("0102 15:04:05.999")
	msg := fmt.Sprintf(format, a...)
	fmt.Printf("%c[1;40;32m%s%c[0m\n", 0x1B, now+" "+msg, 0x1B)
}

func logInfo(format string, a ...interface{}) {
	now := time.Now().Format("0102 15:04:05.999")
	msg := fmt.Sprintf(format, a...)
	fmt.Println(now + " " + msg)
}

func logError(format string, a ...interface{}) {
	now := time.Now().Format("0102 15:04:05.999")
	msg := fmt.Sprintf(format, a...)
	fmt.Printf("%c[1;40;31m%s%c[0m\n", 0x1B, now+" "+msg, 0x1B)
}
