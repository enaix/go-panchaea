package comm

import (
	iof "github.com/enaix/go-panchaea/common/ioformatter"
	"github.com/enaix/go-panchaea/common/proto"
	"github.com/enaix/go-panchaea/common/wumanager"
	"github.com/enaix/go-panchaea/server/decoder"
	"strconv"
)

func FetchWorkUnit(receive *proto.Receive, reply *proto.Reply) bool {
	client, thread, ok := decoder.FindClient(receive, reply)
	if !ok {
		return ok
	}
	workunit, ok := decoder.FindWorkUnit(receive, reply, thread)
	if !ok {
		return ok
	}
	if receive.Error != "" {
		wumanager.ReportError(receive.Error, client, thread, workunit)
		return false
	}
	workunit.Result = receive.Bytecode
	workunit.Status = "completed"
	// TODO tell manager to set wu as completed
	return true
}

func SendWorkUnit(receive *proto.Receive, reply *proto.Reply) bool {
	client, thread, ok := decoder.FindClient(receive, reply)
	if !ok {
		return ok
	}
	// TODO add plugin support
	return true
}
