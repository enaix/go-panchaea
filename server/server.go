package main

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/fatih/color"
	"io/ioutil"
	"net"
	"net/rpc"
	"os"
	"os/exec"
	"path/filepath"
	"plugin"
	"regexp"
	"runtime"
	"strconv"
	"sync"
)

var wg sync.WaitGroup

type Listener int

var Clients []Client

var ClientFile []byte

var Filename string

type Client struct {
	Id      int
	Status  string
	Threads int
}

func NewClient(id int, status string, threads int) {
	Clients = append(Clients, Client{Id: id, Status: status, Threads: threads})
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
	Data     string
	Status   string
	Id       int
	Bytecode []byte
}

type Server interface {
	Init()
	Run(id int) ([]byte, error)
	Prepare(amount int) error
	Process(res [][]byte) error
}

var serv Server

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
	*reply = Reply{Data: Filename, Id: data.Id, Bytecode: ClientFile}
	return nil
}

func (l *Listener) SendWorkUnit(data Receive, reply *Reply) error {
	id := data.Id
	wu, err := serv.Run(id)
	if err != nil {
		printErr(err.Error())
	}
	*reply = Reply{Data: "ok", Id: id, Bytecode: wu}
	return nil
}

func (l *Listener) FetchWorkUnit(data Receive, reply *Reply) error {
	id := data.Id
	// TODO Implement workunit management
	if data.Status == "error" {
		printErr(data.Data)
	}
	*reply = Reply{Data: "ok", Id: id}
	return nil
}

func (l *Listener) SendStatus(data Receive, reply *Reply) error {
	if data.Status == "hello" {
		id := data.Id
		if id == -1 {
			id = len(Clients) + 1
		}
		printSuccess("Client " + strconv.Itoa(id) + " is connected")
		threads, err := strconv.Atoi(data.Data)
		if err != nil {
			printErr(err.Error())
			threads = 1
		}
		NewClient(id, "connected", threads)
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
	Filename = ""
	printWarn("Please provide the client file")
	fmt.Print("    ")
	fmt.Scanln(&Filename)
	f, err := os.Open(Filename)
	if err != nil {
		return err
	}
	defer f.Close()
	ClientFile, err = ioutil.ReadAll(f)
	if err != nil {
		return err
	}
	_, file := filepath.Split(Filename)
	Filename = file
	printSuccess("File is succesfully loaded")
	return nil
}

func initCliConn() (*net.TCPListener, error) {
	printSuccess("Resolving TCP Address...")
	printWarn("Please type in the communication port")
	fmt.Print("    ")
	port := ""
	fmt.Scanln(&port)
	address, err := net.ResolveTCPAddr("tcp", "127.0.0.1:"+port)
	if err != nil {
		return nil, err
	}
	in, err := net.ListenTCP("tcp", address)
	if err != nil {
		return nil, err
	}
	listener := new(Listener)
	rpc.Register(listener)
	return in, nil
}

func processRPC(in *net.TCPListener) {
	defer wg.Done()
	printSuccess("Running..")
	rpc.Accept(in)
}

func buildServer(filename string) (string, error) {
	flag := "-o"
	output := "build.so"
	goexec, err := exec.LookPath("go")
	if err != nil {
		return "", err
	}
	if runtime.GOOS == "windows" {
		return "", errors.New("FATAL: Windows does not support Go Plugins!")
	}
	cmd := exec.Command(goexec, "build", flag, filepath.Join("build", output), "-buildmode=plugin", filename)
	fmt.Println(cmd)
	file_out := filepath.Join("build", output)
	var out, stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	printSuccess("Starting the build process...")
	err = cmd.Run()
	if out.String() != "" {
		fmt.Println(out.String())
	}
	if stderr.String() != "" {
		color.Red(stderr.String())
	}
	if err != nil {
		return file_out, err
	}
	printSuccess("Build is complete!")
	return file_out, nil
}

func initClientServer() (func() interface{}, error) {
	filename := ""
	printWarn("Please provide the server file")
	fmt.Print("    ")
	fmt.Scanln(&filename)
	out, err := buildServer(filename)
	if err != nil {
		return nil, err
	}
	plug, err := plugin.Open(out)
	if err != nil {
		return nil, err
	}
	run, err := plug.Lookup("GetServer")
	if err != nil {
		return nil, err
	}
	GetServer := run.(func() interface{})
	return GetServer, nil
}

func initPluginStruct(GetServer func() interface{}) error {
	servInter := GetServer()
	s, ok := servInter.(Server)
	if !ok {
		return errors.New("Could not receive the server interface!")
	}
	s.Init()
	return nil
}

func main() {
	err := initProject()
	if err != nil {
		printErr(err.Error())
		os.Exit(1)
	}
	in, err := initCliConn()
	if err != nil {
		printErr(err.Error())
		os.Exit(1)
	}
	GetServer, err := initClientServer()
	if err != nil {
		printErr(err.Error())
		os.Exit(1)
	}
	err = initPluginStruct(GetServer)
	if err != nil {
		printErr(err.Error())
		os.Exit(1)
	}
	wg.Add(1)
	go processRPC(in)
	wg.Wait()
}
