package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"github.com/fatih/color"
	"github.com/spf13/viper"
	"log"
	"net/rpc"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
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

var Logger *log.Logger

// TODO add config and log feature
var WUAttempts int // Max failures for one WU, default 2

type Receive struct {
	Data     string
	Status   string
	Id       int
	Bytecode []byte
}

type Reply struct {
	Data     string
	Id       int
	Bytecode []byte
}

type Thread struct {
	Id       int
	Status   string // "ready", "downloading", "uploading", "running", "failed"
	WorkUnit []byte
	Result   []byte
	Attempts int
}

var Threads []Thread

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
	Logger.Println("[E]:    " + err)
}

func printSuccess(s string) {
	if isFormatted(s) {
		color.Green(s)
	} else {
		color.New(color.FgGreen).Print("[*] ")
		fmt.Println(s)
	}
	Logger.Println("[I]:    " + s)
}

func printWarn(s string) {
	if isFormatted(s) {
		color.Yellow(s)
	} else {
		color.New(color.FgYellow).Print("[*] ")
		fmt.Println(s)
	}
	Logger.Println("[W]:    " + s)
}

func console(id int, kill chan bool, input chan string) {
	defer wg.Done()
	for {
		select {
		case <-ctx.Done():
			return
		default:
			cmd := ""
			fmt.Print("[" + strconv.Itoa(id) + "] cli > ")
			// fmt.Scanln(&cmd)
			go func() {
				fmt.Scanln(&input)
			}()
			cmd = <-input
			cmd = strings.TrimSpace(cmd)
			if cmd == "" {
				continue
			} else if cmd == "exit" {
				kill <- true
				close(kill)
				return
			} else if cmd == "help" {
				printWarn("This cli is not implemented") // TODO implement CLI
				printWarn("type `exit` for exit")
			} else {
				printErr("cli: " + cmd + " command not found")
				printErr("    print `help` for help")
			}
		}
	}
}

func initConn(addr, threads string) (*rpc.Client, string, string, error) {
	printSuccess("Connecting to the server...")
	if *overwrite {
		printWarn("Please type in the server ip and port, separated by :")
		addr = ""
		fmt.Print("    ")
		fmt.Scanln(&addr)
	}
	client, err := rpc.Dial("tcp", addr)
	if err != nil {
		return nil, "", "", err
	}
	if *overwrite {
		printWarn("Please type in the number of threads:")
		threads = ""
		fmt.Print("    ")
		fmt.Scanln(&threads)
	}
	_, err = strconv.Atoi(threads)
	if err != nil {
		return nil, "", "", err
	}
	WUAttempts = 2
	return client, addr, threads, nil
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

func sendBytecode(receive Receive, client *rpc.Client) (Reply, error) {
	var reply Reply
	err := client.Call("Listener.FetchWorkUnit", receive, &reply)
	if err != nil {
		return reply, err
	}
	return reply, nil
}

func getBytecode(receive Receive, client *rpc.Client, thread int) (Reply, error) {
	var reply Reply
	err := client.Call("Listener.SendWorkUnit", receive, &reply)
	if err != nil {
		return reply, err
	}
	return reply, nil
}

func reloadBytecode(receive Receive, client *rpc.Client, thread int) (Reply, error) {
	var reply Reply
	err := client.Call("Listener.ReloadWorkUnit", receive, &reply)
	if err != nil {
		return reply, err
	}
	return reply, nil
}

func writeCode(code []byte, filename string) (string, error) {
	if _, err := os.Stat("build"); os.IsNotExist(err) {
		err := os.Mkdir("build", 0755)
		if err != nil {
			printErr(err.Error())
		}
	}
	filename = filepath.Join("build", filename)
	f, err := os.Create(filename)
	if err != nil {
		return filename, err
	}
	defer f.Close()
	_, err = f.Write(code)
	if err != nil {
		return filename, err
	}
	printSuccess("Client code is written!")
	return filename, nil
}

func buildCode(filename string) (string, error) {
	flag := "-o"
	output := "build"
	goexec, err := exec.LookPath("go")
	if err != nil {
		return "", err
	}
	if runtime.GOOS == "windows" {
		flag = "/o"
		output = "build.exe"
		goexec, err = exec.LookPath("go.exe")
		if err != nil {
			return "", err
		}
	}
	cmd := exec.Command(goexec, "build", flag, filepath.Join("build", output), filename)
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

func connect(client *rpc.Client, threads string) (error, []byte, string, int) {
	var reply Reply
	reply, err := sendStatus(Receive{Data: threads, Status: "hello", Id: -1}, client)
	if err != nil {
		return err, nil, "", -1
	}
	if reply.Data == "ok" {
		printSuccess("Connected! Your ID is " + strconv.Itoa(reply.Id))
	} else {
		printErr(reply.Data)
	}
	id := reply.Id
	reply, err = sendStatus(Receive{Data: "", Status: "ready", Id: id}, client)
	if err != nil {
		return err, nil, "", id
	}
	if reply.Data != "ok" {
		printErr(reply.Data)
	}
	printSuccess("Fetching client code...")
	reply, err = fetchCode(Receive{Data: "", Status: "ready", Id: id}, client)
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

func fetchWU(client *rpc.Client, thread *Thread, id int) error {
	rec := Receive{Data: strconv.Itoa(thread.Id), Status: "download", Id: id}
	reply, err := getBytecode(rec, client, thread.Id)
	if err != nil {
		printErr(err.Error())
		thread.Status = "failed"
		return err
	}
	if reply.Data != "ok" {
		printErr("Failed to download WU!")
		thread.Status = "failed"
		return errors.New("WU download failed")
	}
	thread.WorkUnit = reply.Bytecode
	thread.Status = "running"
	return nil
}

func processWU(client *rpc.Client, filename string, thread *Thread, id int) {
	prefix := "./"
	if runtime.GOOS == "windows" {
		prefix = ".\\"
	}
	cmd := exec.Command(prefix+filename, string(thread.WorkUnit))
	var out, stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		thread.Status = "failed"
		rec := Receive{Data: err.Error(), Status: "error " + strconv.Itoa(thread.Id), Id: id}
		sendBytecode(rec, client)
		return
	}
	res := out.Bytes()
	if stderr.String() != "" {
		thread.Status = "failed"
		rec := Receive{Data: stderr.String(), Status: "error " + strconv.Itoa(thread.Id), Id: id}
		sendBytecode(rec, client)
		return
	}
	rec := Receive{Data: strconv.Itoa(thread.Id), Status: "upload", Id: id, Bytecode: res}
	reply, err := sendBytecode(rec, client)
	if reply.Data != "ok" {
		printErr(reply.Data)
	}
	thread.Status = "ready"
	return
}

func reloadWU(client *rpc.Client, thread *Thread, id int) error {
	rec := Receive{Data: strconv.Itoa(thread.Id), Status: "download", Id: id}
	reply, err := reloadBytecode(rec, client, thread.Id)
	if err != nil {
		printErr(err.Error())
		thread.Status = "failed"
		return err
	}
	if reply.Data != "ok" {
		msg := reply.Data
		if msg == "no such wu" {
			printErr("Failed to reload WU: No such WU!")
			thread.Status = "failed"
			return errors.New("WU does not exist!")
		} else if msg == "dead" {
			printErr("Failed to reload WU: Too many failed attempts!")
			thread.Status = "failed"
			return errors.New("WU is dead!")
		} else {
			printErr(msg)
			thread.Status = "failed"
			return errors.New("Unknown WU reload error!")
		}
	}
	thread.WorkUnit = reply.Bytecode
	thread.Status = "running"
	return nil
}

func initThreads(threads int) {
	Threads = make([]Thread, 0)
	for i := 0; i < threads; i++ {
		Threads = append(Threads, Thread{Id: i + 1, Status: "ready"})
	}
}

func initLogger() (*os.File, error) {
	f, err := os.OpenFile("panchaea_client.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return f, err
	}
	l := log.New(f, "[client]", log.Ltime)
	Logger = l
	return f, nil
}

func handleThreads(client *rpc.Client, id int, filename string) error {
	defer wg.Done()
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
			for i, _ := range Threads {
				select {
				case <-ctx.Done():
					return nil
				default:
					if Threads[i].Status == "ready" {
						Threads[i].Attempts = 0
						err := fetchWU(client, &Threads[i], id)
						if err != nil {
							Threads[i].Status = "failed"
							printErr("[" + strconv.Itoa(Threads[i].Id) + "] " + err.Error())
							continue
						}
						Threads[i].Status = "running"
						go processWU(client, filename, &Threads[i], id)
					} else if Threads[i].Status == "failed" {
						if Threads[i].Attempts >= WUAttempts {
							Threads[i].Attempts = 0
							err := fetchWU(client, &Threads[i], id)
							if err != nil {
								Threads[i].Status = "failed"
								printErr("[" + strconv.Itoa(Threads[i].Id) + "] " + err.Error())
								continue
							}
							Threads[i].Status = "running"
							go processWU(client, filename, &Threads[i], id)
							continue
						}
						Threads[i].Attempts++
						err := reloadWU(client, &Threads[i], id)
						if err != nil {
							printErr("[" + strconv.Itoa(Threads[i].Id) + "] " + err.Error())
							Threads[i].Status = "failed"
							Threads[i].Attempts = WUAttempts
							continue
						}
						Threads[i].Status = "running"
						go processWU(client, filename, &Threads[i], id)
					}
				}
			}
		}
	}
}

func initContext(kill chan bool) {
	cont, cls := context.WithTimeout(context.Background(), time.Second)
	ctx = cont
	go func() {
		<-kill
		cls()
	}()
}

func handleInterrupt(kill chan bool) {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
	<-ch
	printErr("Performing clean exit...")
	kill <- true
	close(kill)
}

func handleCleanExit(f *os.File, input chan string) {
	<-ctx.Done()
	f.Close()
	close(input)
}

func initTicker() *time.Ticker {
	tick := time.NewTicker(time.Second)
	return tick
}

func initConfig() (string, string, *viper.Viper) {
	v := viper.New()
	dir, fname := filepath.Split(*config_file)
	if dir == "" {
		dir = "."
	}
	filename := strings.Split(fname, ".")
	v.SetDefault("Addr", "")
	v.SetDefault("Threads", "4")
	v.SetConfigName(filename[0])
	v.SetConfigType(filename[1])
	v.AddConfigPath(dir)
	return "", "4", v
}

func readConfig(v *viper.Viper) (string, string, bool) {
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			printErr("Config file not found!")
			return "", "", false
		}
		printErr("Could not read from the config file!")
		return "", "", false
	}
	addr := v.GetString("Addr")
	threads := v.GetString("Threads")
	return addr, threads, true
}

func writeConfig(v *viper.Viper, addr, threads string) error {
	v.Set("Addr", addr)
	v.Set("Threads", threads)
	err := v.WriteConfigAs(*config_file)
	if err != nil {
		return err
	}
	return nil
}

var (
	config_file = flag.String("config", "panchaea_client.json", "config file location")
	overwrite   = flag.Bool("n", false, "do not read from the config file")
)

func main() {
	logfile, err := initLogger()
	if err != nil {
		fmt.Println("[!] " + err.Error())
		os.Exit(1)
	}
	flag.Parse()
	addr, threads, v := initConfig()
	ok := true
	if !*overwrite {
		addr, threads, ok = readConfig(v)
		if !ok {
			*overwrite = true
		} else {
			printSuccess("Config file (" + *config_file + ") is succesfully loaded")
			printSuccess("tcp_addr: " + addr)
			printSuccess("threads: " + threads)
		}
	}
	client, addr, threads, err := initConn(addr, threads)
	if err != nil {
		printErr(err.Error())
		os.Exit(1)
	}
	thr, err := strconv.Atoi(threads)
	if err != nil {
		printErr(err.Error())
		os.Exit(1)
	}
	if *overwrite {
		err = writeConfig(v, addr, threads)
		if err != nil {
			printErr(err.Error())
		}
	}
	err, bytecode, filename, id := connect(client, threads)
	if err != nil {
		printErr(err.Error())
		os.Exit(1)
	}
	fname, err := writeCode(bytecode, filename)
	if err != nil {
		printErr(err.Error())
		os.Exit(1)
	}
	out, err := buildCode(fname)
	fmt.Println(out)
	initThreads(thr)
	kill := make(chan bool, 1)
	input := make(chan string, 1)
	initContext(kill)
	go handleInterrupt(kill)
	go handleCleanExit(logfile, input)
	wg.Add(2)
	go console(id, kill, input)
	go handleThreads(client, id, out)
	wg.Wait()
}
