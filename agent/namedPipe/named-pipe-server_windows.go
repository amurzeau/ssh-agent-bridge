package namedPipe

import (
	"net"

	"github.com/Microsoft/go-winio"
	"github.com/amurzeau/ssh-agent-bridge/agent"
	"github.com/amurzeau/ssh-agent-bridge/agent/common"
	"github.com/amurzeau/ssh-agent-bridge/log"
)

func ServePipe(pipePath string, queryChannel chan agent.AgentMessageQuery) {
	if pipePath == "" {
		log.Errorf("%s: empty pipe path, skipping serving for ssh-agent queries", PackageName)
		return
	}

	log.Infof("%s: listening for agent requests on pipe %v\n", PackageName, pipePath)

	listenFunction := func() (net.Listener, error) {
		return winio.ListenPipe(pipePath, nil)
	}
	common.GenericNetServer(PackageName, listenFunction, queryChannel)
}
