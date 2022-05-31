package pageantPipe

import (
	"fmt"
	"net"
	"os/user"

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
		var pipeConfig winio.PipeConfig
		user, _ := user.Current()

		pipeConfig.SecurityDescriptor = fmt.Sprintf("O:%sD:(A;;GRGW;;;%s)(D;;GRGW;;;WD)(D;;GRGW;;;NU)", user.Uid, user.Uid)

		log.Debugf("%s: security descriptor: %s", PackageName, pipeConfig.SecurityDescriptor)
		return winio.ListenPipe(pipePath, &pipeConfig)
	}
	common.GenericNetServer(PackageName, listenFunction, ctx)
}
