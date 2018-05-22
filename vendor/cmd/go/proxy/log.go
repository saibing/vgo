// +build !linux

package proxy

import (
	"fmt"
	"time"
)

func logRequest(format string, a ...interface{}) {
	now := time.Now().Format("0102 15:04:05.999")
	msg := fmt.Sprintf(format, a...)
	fmt.Printf("%s\n", now+" "+msg)
}

func logInfo(format string, a ...interface{}) {
	logRequest(format, a...)
}

func logError(format string, a ...interface{}) {
	logRequest(format, a...)
}
