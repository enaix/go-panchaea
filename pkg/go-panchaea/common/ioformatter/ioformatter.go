package ioformatter

import (
	"fmt"
	"github.com/fatih/color"
	"log"
	"os"
	"regexp"
	"sync"
)

var Warnings []string

// Errors contains errors to be sent to the dashboard
var Errors []string

var Reg *regexp.Regexp

var Log *log.Logger

var mut *sync.Mutex

func IsFormatted(s string) bool {
	if Reg.FindString(s) == "" {
		return false
	}
	return true
}

func PrintErr(err string) {
	if IsFormatted(err) {
		color.New(color.FgRed).Fprintf(os.Stderr, err)
		fmt.Println()
		mut.Lock()
		Errors = append(Errors, err)
		mut.Unlock()
	} else {
		color.New(color.BgRed).Add(color.FgWhite).Fprintf(os.Stderr, " ERROR ")
		fmt.Println(" " + err)
		mut.Lock()
		Errors = append(Errors, err)
		mut.Unlock()
	}
	mut.Lock()
	Log.Println("[E]:    " + err)
	mut.Unlock()
}

func PrintSuccess(s string) {
	if IsFormatted(s) {
		color.Green(s)
	} else {
		color.New(color.BgGreen).Add(color.FgWhite).Print(" INFO ")
		fmt.Println(" " + s)
	}
	mut.Lock()
	Log.Println("[I]:    " + s)
	mut.Unlock()
}

func PrintWarn(s string) {
	if IsFormatted(s) {
		color.Yellow(s)
		mut.Lock()
		Warnings = append(Warnings, s)
		mut.Unlock()
	} else {
		color.New(color.BgYellow).Add(color.FgBlack).Print(" WARN ")
		fmt.Println(" " + s)
		mut.Lock()
		Warnings = append(Warnings, s)
		mut.Unlock()
	}
	mut.Lock()
	Log.Println("[W]:    " + s)
	mut.Unlock()
}

func initLogger(filename, prefix string) (*os.File, error) {
	f, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return f, err
	}
	Log = log.New(f, prefix, log.Ltime)
	return f, nil
}

func initFormatter() {
	Reg = regexp.MustCompile(`[\[]+(\w|\W)+[\]]+\s*\w*`)
}

func Init(filename, prefix string, mutex *sync.Mutex) (*os.File, error) {
	mut = mutex
	initFormatter()
	return initLogger(filename, prefix)
}
