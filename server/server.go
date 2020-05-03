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

type Client struct {
	Id     int
	Status string
}

func NewClient(id int, status string) {
	Clients = append(Clients, Client{Id: id, Status: status})
}

func GetClient(id int) (*Client, bool) {
	for i := range Clients {
		if Clients[i].Id == id {
			return &Clients[i], true
		}
	}
	return nil, false
}

type Reply struct {
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
		*reply = Reply{Data: "error", Id: data.Id}
		return errors.New("No input file provided")
	}
	*reply = Reply{Data: "ok", Id: data.Id, Bytecode: ClientFile}
	return nil
}

func (l *Listener) SendStatus(data Receive, reply *Reply) error {
	if data.Status == "hello" {
		id := data.Id
		if id == -1 {
			id = len(Clients) + 1
		}
		printSuccess("Client " + strconv.Itoa(id) + " is connected")
		NewClient(id, "connected")
		*reply = Reply{Data: "ok", Id: id}
	} else if data.Status == "ready" {
		printSuccess("Client " + strconv.Itoa(data.Id) + " is ready")
		cl, ok := GetClient(data.Id)
		if ok {
			cl.Status = "ready"
			*reply = Reply{Data: "ok", Id: data.Id}
		} else {
			printErr("[" + strconv.Itoa(data.Id) + "] " + "Client not found!")
			*reply = Reply{Data: "client not found", Id: data.Id}
		}
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

func initCliConn() error {
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
	err = initCliConn()
	if err != nil {
		printErr(err.Error())
		os.Exit(1)
	}
}
