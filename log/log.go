package log

import (
	"fmt"
	"log"
)

type LogLevel int64

const (
	Debug LogLevel = iota
	Info
	Error
)

var (
	Level                 LogLevel = Info
	UseMessageBoxForFatal bool     = true
)

func Fatalf(format string, v ...any) {
	str := fmt.Sprintf(format, v...)
	outputDebugString(str)
	if UseMessageBoxForFatal {
		messageBox(str)
	}
	log.Print(str)
}

func Errorf(format string, v ...any) {
	if Level <= Error {
		log.Printf("ERROR: "+format, v...)
	}
	outputDebugString(fmt.Sprintf(format, v...))
}

func Infof(format string, v ...any) {
	if Level <= Info {
		log.Printf("info:  "+format, v...)
	}
	outputDebugString(fmt.Sprintf(format, v...))
}

func Debugf(format string, v ...any) {
	if Level <= Debug {
		log.Printf("debug: "+format, v...)
	}
	outputDebugString(fmt.Sprintf(format, v...))
}
