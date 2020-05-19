package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"github.com/fatih/color"
	"io/ioutil"
	"net"
	"net/rpc"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"plugin"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

var wg sync.WaitGroup

var ctx context.Context

var Timeout *time.Duration

// TODO add config and log feature
var WUAttempts int // Max failures for one WU, default 2

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

type WorkUnit struct {
	Data    []byte
	Client  *Client
	Thread  int // 1 to n
	Time    time.Time
	Status  string // "new", "running", "completed", "stuck", "failed", "unknown", "dead"
	Attempt int
	Result  []byte
}

func NewWorkUnit(client *Client, data []byte, thread int) *WorkUnit {
	var wu WorkUnit
	wu.Client = client
	wu.Data = data
	wu.Thread = thread
	wu.Status = "new"
	WorkUnits = append(WorkUnits, wu)
	return &wu
}

func GetWorkUnit(client *Client, thread int) (*WorkUnit, bool) {
	wu := WorkUnit{}
	ok := false
	for i, _ := range WorkUnits {
		select {
		case <-ctx.Done():
			return &WorkUnit{}, false
		default:
			if WorkUnits[i].Client.Id == client.Id && WorkUnits[i].Thread == thread {
				wu = WorkUnits[i]
				ok = true
			}
		}
	}
	return &wu, ok
}

func GetAvailable(client *Client, thread int) (*WorkUnit, bool) {
	for i, _ := range WorkUnits {
		select {
		case <-ctx.Done():
			return &WorkUnit{}, false
		default:
			if WorkUnits[i].Status == "stuck" || WorkUnits[i].Status == "failed" {
				if WorkUnits[i].Attempt >= WUAttempts {
					WorkUnits[i].Status = "dead"
					printErr("FATAL: WorkUnit exceeded all " + strconv.Itoa(WUAttempts) + " attempt(s)")
					continue
				}
				wu := &WorkUnits[i]
				wu.Client = client
				wu.Thread = thread
				wu.Status = "new"
				wu.Attempt++
				return wu, true
			} else if WorkUnits[i].Status == "unknown" {
				if WorkUnits[i].Attempt >= WUAttempts {
					WorkUnits[i].Status = "dead"
					printErr("FATAL: WorkUnit exceeded all " + strconv.Itoa(WUAttempts) + " attempt(s)")
					continue
				}
			}
		}
	}
	return &WorkUnit{}, false
}

var WorkUnits []WorkUnit

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

func (l *Listener) FetchWorkUnit(data Receive, reply *Reply) error {
	id := data.Id
	cli, ok := GetClient(id)
	if !ok {
		printErr("[" + strconv.Itoa(id) + "] " + "Client not found!")
		*reply = Reply{Data: "client not found", Id: id}
		return errors.New("Client not found")
	}
	if data.Status != "upload" {
		thread, err := strconv.Atoi(strings.Split(data.Status, " ")[1])
		if err != nil {
			printErr(err.Error())
			return err
		}
		printErr("[" + strconv.Itoa(id) + "] " + data.Data)
		wu, ok := GetWorkUnit(cli, thread)
		if !ok {
			printErr("[" + strconv.Itoa(id) + "] " + "Error not found! Cannot compute")
			*reply = Reply{Data: "error", Id: id}
			return errors.New("Cannot compute")
		}
		wu.Status = "failed"
		*reply = Reply{Data: "error", Id: id}
		return errors.New(data.Data)
	}
	thread, err := strconv.Atoi(data.Data)
	if err != nil {
		printErr(err.Error())
		*reply = Reply{Data: "error", Id: id}
		return err
	}
	wu, ok := GetWorkUnit(cli, thread)
	if !ok {
		printErr("[" + strconv.Itoa(id) + "] " + "Workunit not found on thread " + strconv.Itoa(thread))
		*reply = Reply{Data: "error", Id: id}
		return errors.New("Workunit not found")
	}
	wu.Status = "completed"
	wu.Result = data.Bytecode
	*reply = Reply{Data: "ok", Id: id}
	return nil
}

func (l *Listener) SendWorkUnit(data Receive, reply *Reply) error {
	id := data.Id
	cli, ok := GetClient(id)
	if !ok {
		printErr("[" + strconv.Itoa(id) + "] " + "Client not found!")
		*reply = Reply{Data: "client not found", Id: id}
		return errors.New("Client not found")
	}
	thread, err := strconv.Atoi(data.Data)
	if err != nil {
		printErr(err.Error())
		*reply = Reply{Data: "error", Id: id}
		return err
	}
	wu, ok := GetAvailable(cli, thread)
	if !ok {
		work, err := serv.Run(id)
		if err != nil {
			printErr(err.Error())
			*reply = Reply{Data: "error", Id: id}
			return err
		}
		wu = NewWorkUnit(cli, work, thread)
	}
	if data.Status == "error" {
		*reply = Reply{Data: "error", Id: id}
		printErr(data.Data)
		return errors.New(data.Data)
	}
	wu.Status = "running"
	*reply = Reply{Data: "ok", Id: id, Bytecode: wu.Data}
	return nil
}

func (l *Listener) ReloadWorkUnit(data Receive, reply *Reply) error {
	id := data.Id
	cli, ok := GetClient(id)
	if !ok {
		printErr("[" + strconv.Itoa(id) + "] " + "Client not found!")
		*reply = Reply{Data: "client not found", Id: id}
		return errors.New("Client not found")
	}
	thread, err := strconv.Atoi(data.Data)
	if err != nil {
		printErr(err.Error())
		*reply = Reply{Data: "error", Id: id}
		return err
	}
	wu, ok := GetWorkUnit(cli, thread)
	if !ok {
		printErr("[" + strconv.Itoa(id) + "] " + "Cannot re-upload: no such WU!")
		*reply = Reply{Data: "no such wu", Id: id}
		return errors.New("Cannot re-upload: no such WU")
	}
	if wu.Attempt >= WUAttempts || wu.Status == "dead" {
		printErr("[" + strconv.Itoa(id) + "] " + "Cannot re-upload: too many failed attempts!")
		*reply = Reply{Data: "dead", Id: id}
		return errors.New("Cannot re-upload: too many failed attempts")
	}
	wu.Attempt++
	wu.Status = "unknown"
	*reply = Reply{Data: "ok", Id: id, Bytecode: wu.Data}
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
	WUAttempts = 2
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
	dur, err := plug.Lookup("Timeout")
	if err != nil {
		return nil, err
	}
	tim := dur.(*time.Duration)
	Timeout = tim
	return GetServer, nil
}

func initPluginStruct(GetServer func() interface{}) error {
	servInter := GetServer()
	s, ok := servInter.(Server)
	if !ok {
		return errors.New("Could not receive the server interface!")
	}
	s.Init()
	serv = s
	return nil
}

func initContext(kill chan bool) {
	cont, cls := context.WithTimeout(context.Background(), time.Second)
	ctx = cont
	go func() {
		<-kill
		cls()
	}()
}

func handleInterrupt(kill chan bool, lis *net.TCPListener) {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
	<-ch
	printErr("Performing clean exit...")
	lis.Close()
	kill <- true
	close(kill)
}

func handleClients(tick *time.Ticker) {
	defer wg.Done()
	for next := range tick.C {
		select {
		case <-ctx.Done():
			tick.Stop()
			return
		default:
			for i, _ := range WorkUnits {
				if WorkUnits[i].Status == "running" || WorkUnits[i].Status == "stuck" {
					WorkUnits[i].Time.Add(time.Second)
				}
				if WorkUnits[i].Time.After(next.Add(*Timeout)) {
					WorkUnits[i].Status = "stuck"
				}
			}
		}
	}
}

func initTicker() *time.Ticker {
	tick := time.NewTicker(time.Second)
	return tick
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
	kill := make(chan bool, 1)
	initContext(kill)
	t := initTicker()
	go handleInterrupt(kill, in)
	wg.Add(2)
	go handleClients(t)
	go processRPC(in)
	wg.Wait()
}
