package namedPipe

import (
	"fmt"
	"net"
	"os/user"

	"github.com/Microsoft/go-winio"
	"github.com/amurzeau/ssh-agent-bridge/agent"
	"github.com/amurzeau/ssh-agent-bridge/agent/common"
	"github.com/amurzeau/ssh-agent-bridge/log"
)

func ServePipe(pipePath string, ctx *agent.AgentContext) {
	if pipePath == "" {
		log.Errorf("%s: empty pipe path, skipping serving for ssh-agent queries", PackageName)
		return
	}

	log.Infof("%s: listening for agent requests on pipe %v\n", PackageName, pipePath)

	listenFunction := func() (net.Listener, error) {
		var pipeConfig winio.PipeConfig
		user, _ := user.Current()

		// See https://docs.microsoft.com/en-us/archive/msdn-magazine/2008/november/access-control-understanding-windows-file-and-registry-permissions
		pipeConfig.SecurityDescriptor = fmt.Sprintf("O:%sD:(A;;GRGW;;;%s)(D;;GRGW;;;WD)(D;;GRGW;;;NU)", user.Uid, user.Uid)

		log.Debugf("%s: security descriptor: %s", PackageName, pipeConfig.SecurityDescriptor)
		return winio.ListenPipe(pipePath, &pipeConfig)
	}
	common.GenericNetServer(PackageName, listenFunction, ctx)
}
