package ioformatter

import (
	"fmt"
	"log"
	"os"
	"regexp"
	"github.com/fatih/color"
)

var Warnings []string

// Errors contains errors to be sent to the dashboard
var Errors []string

var Reg *regexp.Regexp

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
		Errors = append(Errors, err)
	} else {
		color.New(color.FgRed).Fprintf(os.Stderr, "[!] ")
		fmt.Println(err)
		Errors = append(Errors, "[!] "+err)
	}
	log.Println("[E]:    " + err)
}

func PrintSuccess(s string) {
	if IsFormatted(s) {
		color.Green(s)
	} else {
		color.New(color.FgGreen).Print("[*] ")
		fmt.Println(s)
	}
	log.Println("[I]:    " + s)
}

func PrintWarn(s string) {
	if IsFormatted(s) {
		color.Yellow(s)
		Warnings = append(Warnings, s)
	} else {
		color.New(color.FgYellow).Print("[*] ")
		fmt.Println(s)
		Warnings = append(Warnings, "[!] "+s)
	}
	log.Println("[W]:    " + s)
}

func initLogger(filename, prefix string) (*os.File, error) {
	f, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return f, err
	}
	log.SetOutput(f)
	log.SetPrefix(prefix)
	log.SetFlags(log.Ltime)
	return f, nil
}

func initFormatter() {
	Reg = regexp.MustCompile(`[\[]+(\w|\W)+[\]]+\s*\w*`)
}

func Init(filename, prefix string) (*os.File, error) {
	initFormatter()
	return initLogger(filename, prefix)
}


