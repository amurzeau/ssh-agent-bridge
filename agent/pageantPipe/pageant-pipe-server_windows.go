package pageantPipe

import (
	"net"

	"github.com/Microsoft/go-winio"
	"github.com/amurzeau/ssh-agent-bridge/agent"
	"github.com/amurzeau/ssh-agent-bridge/agent/common"
	"github.com/amurzeau/ssh-agent-bridge/log"
)

func ServePageantPipe(ctx *agent.AgentContext) {
	log.Infof("%s: listening for pageant-pipe requests", PackageName)

	pipePath, err := getPageantPipePath()
	if err != nil {
		log.Errorf("%s: failed to get pageant pipe path: %v", PackageName, err)
		return
	}

	// We must hide the pageant pipe path

	listenFunction := func() (net.Listener, error) {
		return winio.ListenPipe(pipePath, nil)
	}
	common.GenericNetServer(PackageName, listenFunction, ctx)
}
