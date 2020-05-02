package main

import (
	"errors"
	"fmt"
	"github.com/fatih/color"
	"io/ioutil"
	"net"
	"net/rpc"
	"os"
	"regexp"
	"strconv"
)

type Listener int

var Clients []Client

var ClientFile []byte

type Client struct { // add Get function with try/catch
	Id     int
	Status string
}

type Reply struct { // add New function to init the struct
	Data     string
	Id       int
	Bytecode []byte
}

type Receive struct {
	Data   string
	Status string
	Id     int
}

func isFormatted(s string) bool {
	re := regexp.MustCompile(`[\[]+(\w|\W)+[\]]+\s*\w*`)
	if re.FindString(s) == "" {
		return false
	}
	return true
}

func printErr(err string) {
	if isFormatted(err) {
		color.New(color.FgRed).Fprintf(os.Stderr, err)
		fmt.Println()
	} else {
		color.New(color.FgRed).Fprintf(os.Stderr, "[!] ")
		fmt.Println(err)
	}
}

func printSuccess(s string) {
	if isFormatted(s) {
		color.Green(s)
	} else {
		color.New(color.FgGreen).Print("[*] ")
		fmt.Println(s)
	}
}

func printWarn(s string) {
	if isFormatted(s) {
		color.Yellow(s)
	} else {
		color.New(color.FgYellow).Print("[*] ")
		fmt.Println(s)
	}
}

func (l *Listener) Init(data Receive, reply *Reply) error {
	if len(ClientFile) == 0 {
		printErr("No client file provided")
		*reply = Reply{"error", data.Id, nil}
		return errors.New("No input file provided")
	}
	*reply = Reply{"ok", data.Id, ClientFile}
	return nil
}

func (l *Listener) SendStatus(data Receive, reply *Reply) error {
	if data.Status == "hello" {
		id := data.Id
		if id == -1 {
			id = len(Clients) + 1
		}
		printSuccess("Client " + strconv.Itoa(id) + " is connected")
		Clients = append(Clients, Client{id, "connected"})
		*reply = Reply{"ok", id, nil}
	} else if data.Status == "ready" {
		printSuccess("Client " + strconv.Itoa(data.Id) + " is ready")
		Clients[data.Id-1].Status = "ready" // TODO add client Get function call
		*reply = Reply{"ok", data.Id, nil}
	} else if data.Status == "error" {
		printErr("[" + strconv.Itoa(data.Id) + "] " + data.Data)
	}
	return nil
}

func initProject() error {
	filename := ""
	printWarn("Please provide the client file")
	fmt.Print("    ")
	fmt.Scanln(&filename)
	f, err := os.Open(filename)
	if err != nil {
		return err
	}
	ClientFile, err = ioutil.ReadAll(f)
	if err != nil {
		return err
	}
	printSuccess("File is succesfully loaded")
	return nil
}

func initConn() error {
	printSuccess("Resolving TCP Address...")
	printWarn("Please type in the communication port")
	fmt.Print("    ")
	port := ""
	fmt.Scanln(&port)
	address, err := net.ResolveTCPAddr("tcp", "127.0.0.1:"+port)
	if err != nil {
		return err
	}
	in, err := net.ListenTCP("tcp", address)
	if err != nil {
		return err
	}
	listener := new(Listener)
	rpc.Register(listener)
	printSuccess("Running...")
	rpc.Accept(in)
	return nil
}

func main() {
	err := initProject()
	if err != nil {
		printErr(err.Error())
		os.Exit(1)
	}
	err = initConn()
	if err != nil {
		printErr(err.Error())
		os.Exit(1)
	}
}
