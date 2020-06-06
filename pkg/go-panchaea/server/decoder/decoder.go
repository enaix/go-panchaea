package decoder

import (
	iof "github.com/enaix/go-panchaea/common/ioformatter"
	"github.com/enaix/go-panchaea/common/proto"
	"github.com/enaix/go-panchaea/common/wumanager"
	"strconv"
)

func FindClient(receive *proto.Receive, reply *proto.Reply) (*wumanager.Client, *wumanager.Thread, bool) {
	client, ok := wumanager.GetClient(receive.ID)
	if !ok {
		iof.PrintErr(strconv.Itoa(receive.ID) + " Client not found!")
		reply.Error = "Client not found!"
		return &wumanager.Client{}, &wumanager.Thread{}, false
	}
	thread, ok := wumanager.GetThread(client, receive.Thread)
	if !ok {
		iof.PrintErr(strconv.Itoa(receive.ID) + " Thread " + strconv.Itoa(receive.Thread) + " not found!")
		reply.Error = "Thread not found!"
		return &wumanager.Client{}, &wumanager.Thread{}, false
	}
	return client, thread
}

func FindWorkUnit(receive *proto.Receive, reply *proto.Reply, thread *wumanager.Thread) (*wumanager.WorkUnit, bool) {
	wu, ok := wumanager.GetWorkUnit(thread)
	if !ok {
		iof.PrintErr(strconv.Itoa(receive.ID) + " Thread " + strconv.Itoa(receive.Thread) + " has no workunits!")
		reply.Error = "Workunit not found!"
		return &wumanager.WorkUnit{}, false
	}
	return wu, true
}
