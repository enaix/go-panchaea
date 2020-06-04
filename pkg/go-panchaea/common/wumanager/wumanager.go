package manager

import (
	"sync"
	"time"
)

var Clients []Client

var WorkUnits []WorkUnit

var mut *sync.Mutex

// Client represents connected client (node)
type Client struct {
	ID      int
	Status  string // "ready", "running", "failed"
	Threads []*Thread
}

// NewClient registers a new connected client
func NewClient(ID int, status string, threads int) {
	mut.Lock()
	Clients = append(Clients, Client{ID: ID, Status: status, Threads: NewThreads(threads)})
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

type Thread struct {
	ID int
	Status string
	WorkUnits []*WorkUnit
}

func NewThreads(amount int) []*Thread {
	res := make([]*Thread, amount)
	for i := range(res) {
		res[i] = &Thread{ID: i + 1, Status: "ready"}
	}
	return res
}

func GetThread(client *Client, ID int) (*Thread, bool) {
	for i := range(client.Threads) {
		if client.Threads[i].ID == ID {
			return client.Threads[i], true
		}
	}
	return nil, false
}

// WorkUnit represents registered WU
type WorkUnit struct {
	Data    []byte
	Client  *Client
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
	wu.Status = "new"
	mut.Lock()
	WorkUnits = append(WorkUnits, wu)
	mut.Unlock()
	return &wu
}

func GetWorkUnit(thread *Thread) *WorkUnit {
	return thread.WorkUnits[len(thread.WorkUnits) - 1]
}

