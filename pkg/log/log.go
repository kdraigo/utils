package log

import (
	"fmt"
	"os"
	"runtime"
	"time"
)

var reset = "\033[0m"
var red = "\033[31m"
var green = "\033[32m"
var yellow = "\033[33m"
var blue = "\033[34m"
var purple = "\033[35m"

var loggerInstance *logger = initLogger()

type logger struct {
	DataFile    *os.File
	InfoFile    *os.File
	WarningFile *os.File
	ErrorFile   *os.File
	FatalFile   *os.File
}

func formatCurrentTimeStamp() string {
	return time.Now().Format(time.RFC3339)
}

func colorMessage(msg string, color string) string {
	return fmt.Sprintf("%v %v %v", color, msg, reset)
}

func creteLogFile(dirname, name string) *os.File {
	formattedName := fmt.Sprintf(
		"%v/%v-%v-log.txt",
		dirname, name, time.Now().Local().Format("2006-01-02"))

	file, err := os.OpenFile(formattedName, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Printf("Error when tying to create file %v", err)
		os.Exit(1)
	}
	return file
}

func checkAndCreateLogDir() error {
	if _, err := os.Stat("logs"); os.IsNotExist(err) {
		return os.Mkdir("logs", os.ModePerm)
	}
	return nil
}

func initLogger() *logger {
	if err := checkAndCreateLogDir(); err != nil {
		fmt.Println("Can't create log dir.")
		os.Exit(1)
	}

	lg := &logger{
		DataFile:    creteLogFile("logs", "data"),
		InfoFile:    creteLogFile("logs", "info"),
		WarningFile: creteLogFile("logs", "warning"),
		ErrorFile:   creteLogFile("logs", "error"),
		FatalFile:   creteLogFile("logs", "fatal"),
	}
	return lg
}

// CleanupLogFiles removes all log files with logs/ dir.
func CleanupLogFiles() {
	os.Remove(loggerInstance.DataFile.Name())
	os.Remove(loggerInstance.InfoFile.Name())
	os.Remove(loggerInstance.WarningFile.Name())
	os.Remove(loggerInstance.ErrorFile.Name())
	os.Remove(loggerInstance.FatalFile.Name())
	os.RemoveAll("logs")
}

func fmtArgs(args ...interface{}) string {
	formatted := ""
	for _, arg := range args {
		formatted += fmt.Sprintf("%v ", arg)
	}
	return formatted
}

// Info log on INFO level.
func Info(args ...interface{}) {
	pc, _, line, _ := runtime.Caller(1)
	msg := fmt.Sprintf(
		"[%v][%v:%v][%v] %v\n",
		"INFO", runtime.FuncForPC(pc).Name(), line, formatCurrentTimeStamp(), fmtArgs(args...))
	fmt.Println(colorMessage(msg, green))
	loggerInstance.InfoFile.WriteString(msg)
}

// InfoF log on INFO level with formatting.
func InfoF(str string, args ...interface{}) {
	pc, _, line, _ := runtime.Caller(1)
	msg := fmt.Sprintf(
		"[%v][%v:%v][%v] %v\n",
		"INFO", runtime.FuncForPC(pc).Name(), line, formatCurrentTimeStamp(), fmt.Sprintf(str, args...))
	fmt.Println(colorMessage(msg, green))
	loggerInstance.InfoFile.WriteString(msg)
}

// Error log on ERROR level.
func Error(args ...interface{}) {
	pc, _, line, _ := runtime.Caller(1)
	msg := fmt.Sprintf(
		"[%v][%v:%v][%v] %v\n",
		"ERROR", runtime.FuncForPC(pc).Name(), line, formatCurrentTimeStamp(), fmtArgs(args...))
	fmt.Println(colorMessage(msg, red))
	loggerInstance.ErrorFile.WriteString(msg)
}

// ErrorF log on error level with formatting.
func ErrorF(str string, args ...interface{}) {
	pc, _, line, _ := runtime.Caller(1)
	msg := fmt.Sprintf(
		"[%v][%v:%v][%v] %v\n",
		"ERROR", runtime.FuncForPC(pc).Name(), line, formatCurrentTimeStamp(), fmt.Sprintf(str, args...))
	fmt.Println(colorMessage(msg, red))
	loggerInstance.ErrorFile.WriteString(msg)
}

// Warn log on WARNING level.
func Warn(args ...interface{}) {
	pc, _, line, _ := runtime.Caller(1)
	msg := fmt.Sprintf(
		"[%v][%v:%v][%v] %v\n",
		"WARNING", runtime.FuncForPC(pc).Name(), line, formatCurrentTimeStamp(), fmtArgs(args...))
	fmt.Println(colorMessage(msg, yellow))
	loggerInstance.WarningFile.WriteString(msg)
}

// WarnF log on WARNING level with formatting.
func WarnF(str string, args ...interface{}) {
	pc, _, line, _ := runtime.Caller(1)
	msg := fmt.Sprintf(
		"[%v][%v:%v][%v] %v\n",
		"WARNING", runtime.FuncForPC(pc).Name(), line, formatCurrentTimeStamp(), fmt.Sprintf(str, args...))
	fmt.Println(colorMessage(msg, yellow))
	loggerInstance.WarningFile.WriteString(msg)
}

// Data log on DATA level.
func Data(args ...interface{}) {
	pc, _, line, _ := runtime.Caller(1)
	msg := fmt.Sprintf(
		"[%v][%v:%v][%v] %v\n",
		"DATA", runtime.FuncForPC(pc).Name(), line, formatCurrentTimeStamp(), fmtArgs(args...))
	fmt.Println(colorMessage(msg, blue))
	loggerInstance.DataFile.WriteString(msg)

}

// DataF log on DATA level with formatting.
func DataF(str string, args ...interface{}) {
	pc, _, line, _ := runtime.Caller(1)
	msg := fmt.Sprintf(
		"[%v][%v:%v][%v] %v\n",
		"DATA", runtime.FuncForPC(pc).Name(), line, formatCurrentTimeStamp(), fmt.Sprintf(str, args...))
	fmt.Println(colorMessage(msg, blue))
	loggerInstance.DataFile.WriteString(msg)
}

// Fatal log on FATAL level.
func Fatal(args ...interface{}) {
	pc, _, line, _ := runtime.Caller(1)
	msg := fmt.Sprintf(
		"[%v][%v:%v][%v] %v\n",
		"FATAL", runtime.FuncForPC(pc).Name(), line, formatCurrentTimeStamp(), fmtArgs(args...))
	fmt.Println(colorMessage(msg, purple))
	loggerInstance.FatalFile.WriteString(msg)
	os.Exit(1)
}

// FatalF log on FATAL level with formatting.
func FatalF(str string, args ...interface{}) {
	pc, _, line, _ := runtime.Caller(1)
	msg := fmt.Sprintf(
		"[%v][%v:%v][%v] %v\n",
		"FATAL", runtime.FuncForPC(pc).Name(), line, formatCurrentTimeStamp(), fmt.Sprintf(str, args...))
	fmt.Println(colorMessage(msg, purple))
	loggerInstance.FatalFile.WriteString(msg)
	os.Exit(1)
}
