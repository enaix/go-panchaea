package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"html"
	"io/ioutil"
	"log"
	"net"
	"net/http"
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

	"github.com/fatih/color"
	"github.com/spf13/viper"
)

var wg sync.WaitGroup

var mut sync.Mutex

var ctx context.Context

// Timeout before the workunit is consIDered to be stuck
var Timeout *time.Duration

var reg *regexp.Regexp

// WUAttempts declares max failures for one WU, default 2
var WUAttempts int

// Listener RPC (int?)
type Listener int

// Clients contains connected clients
var Clients []Client

// ClientFile stores the code to be sent to the clients
var ClientFile []byte

// Filename of the code file
var Filename string

// Status of the server (ready, running, failed)
var Status string

var finished chan bool

// Client represents connected client (node)
type Client struct {
	ID      int
	Status  string // "ready", "running", "failed"
	Threads int
}

// NewClient registers a new connected client
func NewClient(ID int, status string, threads int) {
	mut.Lock()
	Clients = append(Clients, Client{ID: ID, Status: status, Threads: threads})
	mut.Unlock()
}

// GetClient returns the client by ID
func GetClient(ID int) (*Client, bool) {
	for i := range Clients {
		if Clients[i].ID == ID {
			return &Clients[i], true
		}
	}
	return nil, false
}

// WorkUnit represents registered WU
type WorkUnit struct {
	Data    []byte
	Client  *Client
	Thread  int // 1 to n
	Time    time.Time
	Status  string // "new", "running", "completed", "stuck", "failed", "unknown", "dead"
	Attempt int
	Result  []byte
}

// NewWorkUnit registers a new WU
func NewWorkUnit(client *Client, data []byte, thread int) *WorkUnit {
	var wu WorkUnit
	wu.Client = client
	wu.Data = data
	wu.Thread = thread
	wu.Status = "new"
	mut.Lock()
	WorkUnits = append(WorkUnits, &wu)
	mut.Unlock()
	return &wu
}

// GetWorkUnit returns the WU by the client and its thread
func GetWorkUnit(client *Client, thread int) (*WorkUnit, bool) {
	wu := &WorkUnit{}
	ok := false
	for i := range WorkUnits {
		select {
		case <-ctx.Done():
			return &WorkUnit{}, false
		default:
			if WorkUnits[i].Client.ID == client.ID && WorkUnits[i].Thread == thread {
				wu = WorkUnits[i]
				ok = true
			}
		}
	}
	return wu, ok
}

// GetAvailable returns available WU and assigns it to the client
func GetAvailable(client *Client, thread int) (*WorkUnit, bool) {
	for i := range WorkUnits {
		select {
		case <-ctx.Done():
			return &WorkUnit{}, false
		default:
			if WorkUnits[i].Status == "stuck" || WorkUnits[i].Status == "failed" {
				if WorkUnits[i].Attempt >= WUAttempts {
					mut.Lock()
					WorkUnits[i].Status = "dead"
					mut.Unlock()
					printErr("FATAL: WorkUnit exceeded all " + strconv.Itoa(WUAttempts) + " attempt(s)")
					continue
				}
				mut.Lock()
				wu := *WorkUnits[i]
				wu.Client = client
				wu.Thread = thread
				wu.Status = "new"
				wu.Attempt++
				mut.Unlock()
				return &wu, true
			} else if WorkUnits[i].Status == "unknown" {
				if WorkUnits[i].Attempt >= WUAttempts {
					mut.Lock()
					WorkUnits[i].Status = "dead"
					mut.Unlock()
					printErr("FATAL: WorkUnit exceeded all " + strconv.Itoa(WUAttempts) + " attempt(s)")
					continue
				}
			}
		}
	}
	return &WorkUnit{}, false
}

// WorkUnits contains all WUs
var WorkUnits []*WorkUnit

// Reply contains data to be sent to a client
type Reply struct {
	Data     string
	ID       int
	Bytecode []byte
}

// Receive contains data to be fetched from a client
type Receive struct {
	Data     string
	Status   string
	ID       int
	Bytecode []byte
}

// Server represents the reflection of the plugin's Server struct
type Server interface {
	Init()
	Run(ID int) ([]byte, error)
	Prepare(amount int) error
	Process(res [][]byte) error
}

var serv Server

// APIResponse contains data to be sent to the dashboard
type APIResponse struct {
	Warnings  []string
	Errors    []string
	Status    string
	Clients   *[]Client
	WorkUnits []*WorkUnit
}

var apiresp APIResponse

// Warnings contains warnings to be sent to the dashboard
var Warnings []string

// Errors contains errors to be sent to the dashboard
var Errors []string

func isFormatted(s string) bool {
	if reg.FindString(s) == "" {
		return false
	}
	return true
}

func printErr(err string) {
	if isFormatted(err) {
		color.New(color.FgRed).Fprintf(os.Stderr, err)
		fmt.Println()
		Errors = append(Errors, err)
	} else {
		color.New(color.FgRed).Fprintf(os.Stderr, "[!] ")
		fmt.Println(err)
		mut.Lock()
		Errors = append(Errors, "[!] "+err)
	}
	log.Println("[E]:    " + err)
}

func printSuccess(s string) {
	if isFormatted(s) {
		color.Green(s)
	} else {
		color.New(color.FgGreen).Print("[*] ")
		fmt.Println(s)
	}
	log.Println("[I]:    " + s)
}

func printWarn(s string) {
	if isFormatted(s) {
		color.Yellow(s)
		Warnings = append(Warnings, s)
	} else {
		color.New(color.FgYellow).Print("[*] ")
		fmt.Println(s)
		Warnings = append(Warnings, "[!] "+s)
	}
	log.Println("[W]:    " + s)
}

// Init sends the ClientFile to a client
func (l *Listener) Init(data Receive, reply *Reply) error {
	if len(ClientFile) == 0 {
		printErr("No client file provided")
		*reply = Reply{Data: "error", ID: data.ID}
		return errors.New("No input file provided")
	}
	*reply = Reply{Data: Filename, ID: data.ID, Bytecode: ClientFile}
	return nil
}

// Finish preapres WUs result and calls the Prepare server function
func Finish() error {
	tick := 0
	printSuccess("Waiting for the clients to finish WUs...")
	for {
		select {
		case <-ctx.Done():
			printErr("Writing WUs data to the log, please do not abort the process")
			for i := range WorkUnits {
				log.Println("[E] Not completed WU, id: " + strconv.Itoa(i) + "; please re-run it manually")
				log.Println("---------------[start JSON data]---------------")
				log.Println(string(WorkUnits[i].Data))
				log.Println("----------------[end JSON data]----------------")
				log.Println("---------------[start JSON result]---------------")
				log.Println(string(WorkUnits[i].Result))
				log.Println("----------------[end JSON result]----------------")
			}
			return errors.New("Finishing process is terminated by the user!")
		default:
			computing := 0
			stuck := 0
			for i := range Clients {
				if Clients[i].Status == "running" {
					computing++
				} else if Clients[i].Status == "stuck" || Clients[i].Status == "unknown" {
					stuck++
				}
			}
			tick++
			if computing == 0 {
				if stuck != 0 {
					tmp := ""
					printWarn("There are " + strconv.Itoa(stuck) + " stuck clients, ignore and finish the job? [Y/n]")
					fmt.Print("    ")
					fmt.Scanln(&tmp)
					if tmp == "n" || tmp == "N" {
						continue
					} else {
						break
					}
				}
				break
			}
			if tick%100 == 0 {
				tmp := ""
				printWarn("Clients may be stuck, ignore and finish the job? [y/N]")
				fmt.Print("    ")
				fmt.Scanln(&tmp)
				if tmp == "y" || tmp == "Y" {
					break
				} else {
					continue
				}
			}
			time.Sleep(time.Second)
		}
	}
	var ok, run, stuck, fail int
	res := make([][]byte, 0)
	Status = "FINISH"
	for i := range WorkUnits {
		res = append(res, WorkUnits[i].Result)
		switch WorkUnits[i].Status { // "new", "running", "completed", "stuck", "failed", "unknown", "dead"
		case "completed":
			ok++
		case "new":
			log.Println("[E] Not completed WU, id: " + strconv.Itoa(i) + "; please re-run it manually")
			log.Println("---------------[start JSON data]---------------")
			log.Println(string(WorkUnits[i].Data))
			log.Println("----------------[end JSON data]----------------")
			run++
		case "running":
			log.Println("[E] Not completed WU, id: " + strconv.Itoa(i) + "; please re-run it manually")
			log.Println("---------------[start JSON data]---------------")
			log.Println(string(WorkUnits[i].Data))
			log.Println("----------------[end JSON data]----------------")
			run++
		case "stuck":
			log.Println("[E] Stuck WU, id: " + strconv.Itoa(i) + "; please re-run it manually")
			log.Println("---------------[start JSON data]---------------")
			log.Println(string(WorkUnits[i].Data))
			log.Println("----------------[end JSON data]----------------")
			stuck++
		case "failed":
			log.Println("[E] Failed WU, id: " + strconv.Itoa(i) + "; please re-run it manually")
			log.Println("---------------[start JSON data]---------------")
			log.Println(string(WorkUnits[i].Data))
			log.Println("----------------[end JSON data]----------------")
			fail++
		case "unknown":
			log.Println("[E] Failed WU, id: " + strconv.Itoa(i) + "; please re-run it manually")
			log.Println("---------------[start JSON data]---------------")
			log.Println(string(WorkUnits[i].Data))
			log.Println("----------------[end JSON data]----------------")
			fail++
		case "dead":
			log.Println("[E] Failed WU, id: " + strconv.Itoa(i) + "; please re-run it manually")
			log.Println("---------------[start JSON data]---------------")
			log.Println(string(WorkUnits[i].Data))
			log.Println("----------------[end JSON data]----------------")
			fail++
		}
	}
	printWarn("WARN: " + strconv.Itoa(ok) + " WUs completed, " + strconv.Itoa(run) + " in process, " + strconv.Itoa(stuck) + " stuck and " + strconv.Itoa(fail) + " failed.")
	printWarn("Failed WUs info will appear in the log file")
	err := serv.Process(res)
	if err != nil {
		printErr(err.Error())
		return err
	}
	return nil
}

// FetchWorkUnit gets the completed WU from a client
func (l *Listener) FetchWorkUnit(data Receive, reply *Reply) error {
	ID := data.ID
	cli, ok := GetClient(ID)
	if !ok {
		printErr("[" + strconv.Itoa(ID) + "] " + "Client not found!")
		*reply = Reply{Data: "client not found", ID: ID}
		return errors.New("Client not found")
	}
	if data.Status != "upload" {
		thread, err := strconv.Atoi(strings.Split(data.Status, " ")[1])
		if err != nil {
			printErr(err.Error())
			return err
		}
		printErr("[" + strconv.Itoa(ID) + "] " + data.Data)
		wu, ok := GetWorkUnit(cli, thread)
		if !ok {
			printErr("[" + strconv.Itoa(ID) + "] " + "Error not found! Cannot compute")
			*reply = Reply{Data: "error", ID: ID}
			return errors.New("Cannot compute")
		}
		wu.Status = "failed"
		*reply = Reply{Data: "error", ID: ID}
		return errors.New(data.Data)
	}
	thread, err := strconv.Atoi(data.Data)
	if err != nil {
		printErr(err.Error())
		*reply = Reply{Data: "error", ID: ID}
		return err
	}
	wu, ok := GetWorkUnit(cli, thread)
	if !ok {
		printErr("[" + strconv.Itoa(ID) + "] " + "Workunit not found on thread " + strconv.Itoa(thread))
		*reply = Reply{Data: "error", ID: ID}
		return errors.New("Workunit not found")
	}
	mut.Lock()
	wu.Status = "completed"
	wu.Result = data.Bytecode
	mut.Unlock()
	*reply = Reply{Data: "ok", ID: ID}
	return nil
}

// SendWorkUnit sends the WU to a client
func (l *Listener) SendWorkUnit(data Receive, reply *Reply) error {
	ID := data.ID
	cli, ok := GetClient(ID)
	if !ok {
		printErr("[" + strconv.Itoa(ID) + "] " + "Client not found!")
		*reply = Reply{Data: "client not found", ID: ID}
		return errors.New("Client not found")
	}
	thread, err := strconv.Atoi(data.Data)
	if err != nil {
		printErr(err.Error())
		*reply = Reply{Data: "error", ID: ID}
		return err
	}
	wu, ok := GetAvailable(cli, thread)
	if !ok {
		work, err := serv.Run(ID)
		if err != nil {
			printErr(err.Error()) // No more WUs, finishing...
			finished <- true
			*reply = Reply{Data: "error", ID: ID}
			return err
		}
		wu = NewWorkUnit(cli, work, thread)
	}
	if data.Status == "error" {
		*reply = Reply{Data: "error", ID: ID}
		printErr(data.Data)
		return errors.New(data.Data)
	}
	mut.Lock()
	wu.Status = "running"
	mut.Unlock()
	*reply = Reply{Data: "ok", ID: ID, Bytecode: wu.Data}
	return nil
}

// ReloadWorkUnit sends the WU again if necessary
func (l *Listener) ReloadWorkUnit(data Receive, reply *Reply) error {
	ID := data.ID
	cli, ok := GetClient(ID)
	if !ok {
		printErr("[" + strconv.Itoa(ID) + "] " + "Client not found!")
		*reply = Reply{Data: "client not found", ID: ID}
		return errors.New("Client not found")
	}
	thread, err := strconv.Atoi(data.Data)
	if err != nil {
		printErr(err.Error())
		*reply = Reply{Data: "error", ID: ID}
		return err
	}
	wu, ok := GetWorkUnit(cli, thread)
	if !ok {
		printErr("[" + strconv.Itoa(ID) + "] " + "Cannot re-upload: no such WU!")
		*reply = Reply{Data: "no such wu", ID: ID}
		return errors.New("Cannot re-upload: no such WU")
	}
	if wu.Attempt >= WUAttempts || wu.Status == "dead" {
		printErr("[" + strconv.Itoa(ID) + "] " + "Cannot re-upload: too many failed attempts!")
		*reply = Reply{Data: "dead", ID: ID}
		return errors.New("Cannot re-upload: too many failed attempts")
	}
	mut.Lock()
	wu.Attempt++
	wu.Status = "unknown"
	mut.Unlock()
	*reply = Reply{Data: "ok", ID: ID, Bytecode: wu.Data}
	return nil
}

// SendStatus gets client's status
func (l *Listener) SendStatus(data Receive, reply *Reply) error {
	if data.Status == "hello" {
		ID := data.ID
		if ID == -1 {
			ID = len(Clients) + 1
		}
		printSuccess("Client " + strconv.Itoa(ID) + " is connected")
		threads, err := strconv.Atoi(data.Data)
		if err != nil {
			printErr(err.Error())
			threads = 1
		}
		NewClient(ID, "ready", threads)
		*reply = Reply{Data: "ok", ID: ID}
	} else if data.Status == "ready" {
		printSuccess("Client " + strconv.Itoa(data.ID) + " is ready")
		cl, ok := GetClient(data.ID)
		if ok {
			mut.Lock()
			cl.Status = "ready"
			mut.Unlock()
			*reply = Reply{Data: "ok", ID: data.ID}
		} else {
			printErr("[" + strconv.Itoa(data.ID) + "] " + "Client not found!")
			*reply = Reply{Data: "client not found", ID: data.ID}
		}
	} else if data.Status == "error" {
		cl, ok := GetClient(data.ID)
		if ok {
			cl.Status = "failed"
		}
		printErr("[" + strconv.Itoa(data.ID) + "] " + data.Data)
	}
	return nil
}

func initProject(client_file string) (error, string) {
	if *overwrite {
		Filename = ""
		printWarn("Please provide the client file")
		fmt.Print("    ")
		fmt.Scanln(&Filename)
		client_file = Filename
	}
	f, err := os.Open(client_file)
	if err != nil {
		return err, ""
	}
	defer f.Close()
	ClientFile, err = ioutil.ReadAll(f)
	if err != nil {
		return err, ""
	}
	_, file := filepath.Split(client_file)
	Filename = file
	printSuccess("File is succesfully loaded")
	WUAttempts = 2
	return nil, client_file
}

func initCliConn(port string) (*net.TCPListener, string, error) {
	printSuccess("Resolving TCP Address...")
	if *overwrite {
		printWarn("Please type in the communication port")
		fmt.Print("    ")
		port = ""
		fmt.Scanln(&port)
	}
	address, err := net.ResolveTCPAddr("tcp", "127.0.0.1:"+port)
	if err != nil {
		return nil, "", err
	}
	in, err := net.ListenTCP("tcp", address)
	if err != nil {
		return nil, "", err
	}
	listener := new(Listener)
	rpc.Register(listener)
	return in, port, nil
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
	debugParam := ""
	if err != nil {
		return "", err
	}
	if runtime.GOOS == "windows" {
		return "", errors.New("FATAL: Windows does not support Go Plugins!")
	}
	if *debug {
		debugParam = " -gcflags='all=-N -l'"
	}
	cmd := exec.Command(goexec, "build", flag, filepath.Join("build", output), "-buildmode=plugin"+debugParam, filename)
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

func initClientServer(server_file string) (func() interface{}, string, error) {
	if *overwrite {
		filename := ""
		printWarn("Please provide the server file")
		fmt.Print("    ")
		fmt.Scanln(&filename)
		server_file = filename
	}
	out, err := buildServer(server_file)
	if err != nil {
		return nil, "", err
	}
	plug, err := plugin.Open(out)
	if err != nil {
		return nil, "", err
	}
	run, err := plug.Lookup("GetServer")
	if err != nil {
		return nil, "", err
	}
	GetServer := run.(func() interface{})
	dur, err := plug.Lookup("Timeout")
	if err != nil {
		return nil, "", err
	}
	tim := dur.(*time.Duration)
	Timeout = tim
	return GetServer, server_file, nil
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

func initLogger() (*os.File, error) {
	f, err := os.OpenFile("panchaea_server.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return f, err
	}
	log.SetOutput(f)
	log.SetPrefix("[server]")
	log.SetFlags(log.Ltime)
	return f, nil
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

func handleCleanExit(f *os.File, webserver *http.Server) {
	<-ctx.Done()
	f.Close()
	printErr("Writing WUs data to the log, please do not abort the process")
	for i := range WorkUnits {
		log.Println("[E] Not completed WU, id: " + strconv.Itoa(i) + "; please re-run it manually")
		log.Println("---------------[start JSON data]---------------")
		log.Println(string(WorkUnits[i].Data))
		log.Println("----------------[end JSON data]----------------")
		log.Println("---------------[start JSON result]---------------")
		log.Println(string(WorkUnits[i].Result))
		log.Println("----------------[end JSON result]----------------")
	}
	close(finished)
	// err := webserver.Shutdown(ctx)
	// if err != nil {
	// 	printErr(err.Error())
	// }
}
func handleFinish() {
	defer wg.Done()
	<-finished
	err := Finish()
	if err != nil {
		printErr(err.Error())
	}
}

func handleClients(tick *time.Ticker) {
	defer wg.Done()
	for next := range tick.C {
		select {
		case <-ctx.Done():
			tick.Stop()
			return
		default:
			ok := false
			for i := range WorkUnits {
				if WorkUnits[i].Status == "running" || WorkUnits[i].Status == "stuck" {
					WorkUnits[i].Time.Add(time.Second)
					Status = "RUNNING"
					ok = true
				}
				if WorkUnits[i].Time.After(next.Add(*Timeout)) {
					WorkUnits[i].Status = "stuck"
				}
				if !ok {
					Status = "FAILED"
				}
			}
		}
	}
}

func initTicker() *time.Ticker {
	tick := time.NewTicker(time.Second)
	return tick
}

func initConfig() (string, string, string, string, *viper.Viper) {
	v := viper.New()
	dir, fname := filepath.Split(*config_file)
	if dir == "" {
		dir = "."
	}
	filename := strings.Split(fname, ".")
	v.SetDefault("ClientFile", "")
	v.SetDefault("Port", "0")
	v.SetDefault("ServerFile", "")
	v.SetDefault("DashboardPort", "0")
	v.SetConfigName(filename[0])
	v.SetConfigType(filename[1])
	v.AddConfigPath(dir)
	return "", "0", "", "0", v
}

func readConfig(v *viper.Viper) (string, string, string, string, bool) {
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			printErr("Config file not found!")
			return "", "", "", "", false
		}
		printErr("Could not read from the config file!")
		return "", "", "", "", false
	}
	client_file := v.GetString("ClientFile")
	port := v.GetString("Port")
	server_file := v.GetString("ServerFile")
	dashboard_port := v.GetString("DashboardPort")
	return client_file, port, server_file, dashboard_port, true
}

func writeConfig(v *viper.Viper, client_file, port, server_file, dashboard_port string) error {
	v.Set("ClientFile", client_file)
	v.Set("Port", port)
	v.Set("ServerFile", server_file)
	v.Set("DashboardPort", dashboard_port)
	err := v.WriteConfigAs(*config_file)
	if err != nil {
		return err
	}
	return nil
}

func initAPI() {
	apiresp = APIResponse{Warnings: Warnings, Errors: Errors, Clients: &Clients, WorkUnits: WorkUnits}
}

func updateAPI() {
	warn := Warnings
	err := Errors
	for i := range warn {
		warn[i] = html.EscapeString(warn[i])
	}
	for i := range err {
		err[i] = html.EscapeString(err[i])
	}
	apiresp.Warnings = warn
	apiresp.Errors = err
	apiresp.WorkUnits = WorkUnits
	apiresp.Status = Status
	Warnings = []string{}
	Errors = []string{}
}

func handleAPI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	updateAPI()
	json.NewEncoder(w).Encode(&apiresp)
}

func initDashboard(port string) (string, *http.Server) {
	if *overwrite {
		port = ""
		printWarn("Please provIDe the web dashboard port")
		fmt.Print("    ")
		fmt.Scanln(&port)
	}
	fs := http.FileServer(http.Dir("./dashboard"))
	mux := http.NewServeMux()
	mux.Handle("/", fs)
	mux.HandleFunc("/api", handleAPI)
	webserver := &http.Server{Handler: mux, Addr: ":" + port}
	return port, webserver
}

func handleDashboard(webserver *http.Server) {
	defer wg.Done()
	err := webserver.ListenAndServe()
	if err != nil {
		printErr(err.Error())
	}
}

var (
	config_file = flag.String("config", "panchaea_server.json", "config file location")
	overwrite   = flag.Bool("n", false, "do not read from the config file")
	debug       = flag.Bool("d", false, "delve debug support for plugins")
)

func main() {
	logfile, err := initLogger()
	if err != nil {
		fmt.Println("[!] " + err.Error())
		os.Exit(1)
	}
	re := regexp.MustCompile(`[\[]+(\w|\W)+[\]]+\s*\w*`)
	reg = re
	flag.Parse()
	client_file, port, server_file, dashboard_port, v := initConfig()
	ok := true
	if !*overwrite {
		client_file, port, server_file, dashboard_port, ok = readConfig(v)
		if !ok {
			*overwrite = true
		} else {
			printSuccess("Config file (" + *config_file + ") is succesfully loaded")
			printSuccess("client_file: " + client_file)
			printSuccess("tcp_port: " + port)
			printSuccess("server_file: " + server_file)
			printSuccess("dashboard_port: " + dashboard_port)
		}
	}
	err, client_file = initProject(client_file)
	if err != nil {
		printErr(err.Error())
		os.Exit(1)
	}
	in, port, err := initCliConn(port)
	if err != nil {
		printErr(err.Error())
		os.Exit(1)
	}
	GetServer, server_file, err := initClientServer(server_file)
	if err != nil {
		printErr(err.Error())
		os.Exit(1)
	}
	err = initPluginStruct(GetServer)
	if err != nil {
		printErr(err.Error())
		os.Exit(1)
	}
	initAPI()
	dashboard_port, webserver := initDashboard(dashboard_port)
	if *overwrite {
		err = writeConfig(v, client_file, port, server_file, dashboard_port)
		if err != nil {
			printErr(err.Error())
		}
	}
	Status = "READY"
	kill := make(chan bool, 1)
	finished = make(chan bool, 1)
	initContext(kill)
	t := initTicker()
	go handleInterrupt(kill, in)
	go handleCleanExit(logfile, webserver)
	wg.Add(4)
	go handleDashboard(webserver)
	go handleClients(t)
	go processRPC(in)
	go handleFinish()
	wg.Wait()
}
