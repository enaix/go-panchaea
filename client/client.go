package main

import (
	"fmt"
	"github.com/fatih/color"
	"net/rpc"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
)

var wg sync.WaitGroup

type Receive struct {
	Data   string
	Status string
	Id     int
}

type Reply struct {
	Data     string
	Id       int
	Bytecode []byte
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

func console(id int) {
	defer wg.Done()
	for {
		cmd := ""
		fmt.Print("[" + strconv.Itoa(id) + "] cli > ")
		fmt.Scanln(&cmd)
		cmd = strings.TrimSpace(cmd)
		if cmd == "" {
			continue
		} else if cmd == "exit" {
			// TODO add exit func
			return
		} else if cmd == "help" {
			printWarn("This cli is not implemented")
			printWarn("type `exit` for exit")
		} else {
			printErr("cli: " + cmd + " command not found")
			printErr("    print `help` for help")
		}
	}
}

func initConn() (*rpc.Client, error) {
	printSuccess("Connecting to the server...")
	printWarn("Please type in the server ip and port, separated by :")
	addr := ""
	fmt.Print("    ")
	fmt.Scanln(&addr)
	client, err := rpc.Dial("tcp", addr)
	if err != nil {
		return nil, err
	}
	return client, nil
}

func sendStatus(receive Receive, client *rpc.Client) (Reply, error) {
	var reply Reply
	err := client.Call("Listener.SendStatus", receive, &reply)
	if err != nil {
		return reply, err
	}
	return reply, nil
}

func fetchCode(receive Receive, client *rpc.Client) (Reply, error) {
	var reply Reply
	err := client.Call("Listener.Init", receive, &reply)
	if err != nil {
		return reply, err
	}
	return reply, nil
}

func writeCode(code []byte, filename string) error {
	f, err := os.Create(filename)
	// TODO add project folder
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(code)
	if err != nil {
		return err
	}
	printSuccess("Client code is written!")
	return nil
}

func connect(client *rpc.Client) (error, []byte, string, int) {
	var reply Reply
	reply, err := sendStatus(Receive{"", "hello", -1}, client)
	if err != nil {
		return err, nil, "", -1
	}
	if reply.Data == "ok" {
		printSuccess("Connected! Your ID is " + strconv.Itoa(reply.Id))
	} else {
		printErr(reply.Data)
	}
	id := reply.Id
	reply, err = sendStatus(Receive{"", "ready", id}, client)
	if err != nil {
		return err, nil, "", id
	}
	if reply.Data != "ok" {
		printErr(reply.Data)
	}
	printSuccess("Fetching client code...")
	reply, err = fetchCode(Receive{"", "ready", id}, client)
	if err != nil {
		return err, nil, "", id
	}
	if reply.Data == "error" {
		printErr("Could not fetch the client file")
	} else {
		printSuccess("Code is downloaded!")
	}
	return nil, reply.Bytecode, reply.Data, id
}

func main() {
	client, err := initConn()
	if err != nil {
		printErr(err.Error())
		os.Exit(1)
	}
	err, bytecode, filename, id := connect(client)
	if err != nil {
		printErr(err.Error())
		os.Exit(1)
	}
	err = writeCode(bytecode, filename)
	if err != nil {
		printErr(err.Error())
		os.Exit(1)
	}
	wg.Add(1)
	go console(id)
	wg.Wait()
}
