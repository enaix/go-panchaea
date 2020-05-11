package main

import (
	"bytes"
	"fmt"
	"github.com/fatih/color"
	"net/rpc"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
)

var wg sync.WaitGroup

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

func initConn() (*rpc.Client, string, error) {
	printSuccess("Connecting to the server...")
	printWarn("Please type in the server ip and port, separated by :")
	addr := ""
	fmt.Print("    ")
	fmt.Scanln(&addr)
	client, err := rpc.Dial("tcp", addr)
	if err != nil {
		return nil, "", err
	}
	printWarn("Please type in the number of threads:")
	threads := ""
	fmt.Print("    ")
	fmt.Scanln(&threads)
	_, err = strconv.Atoi(threads)
	if err != nil {
		return nil, "", err
	}
	return client, threads, nil
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

func processWU(client *rpc.Client, filename string, wu []byte, thread, id int) {
	prefix := "./"
	if runtime.GOOS == "windows" {
		prefix = ".\\"
	}
	cmd := exec.Command(prefix+filename, string(wu))
	var out, stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		rec := Receive{Data: err.Error(), Status: "error", Id: id}
		sendBytecode(rec, client)
		return
	}
	res := out.Bytes()
	if stderr.String() != "" {
		rec := Receive{Data: stderr.String(), Status: "error", Id: id}
		sendBytecode(rec, client)
		return
	}
	rec := Receive{Data: strconv.Itoa(thread), Status: "upload", Id: id, Bytecode: res}
	reply, err := sendBytecode(rec, client)
	if reply.Data != "ok" {
		printErr(reply.Data)
	}
	return
}

func main() {
	client, threads, err := initConn()
	if err != nil {
		printErr(err.Error())
		os.Exit(1)
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
	wg.Add(1)
	go console(id)
	wg.Wait()
}
