package pageantPipe

import (
	"errors"
	"fmt"
	"net"

	"github.com/Microsoft/go-winio"
	"github.com/amurzeau/ssh-agent-bridge/agent"
	"github.com/amurzeau/ssh-agent-bridge/agent/common"
	"github.com/amurzeau/ssh-agent-bridge/log"
)

func ClientPageantPipe(ctx *agent.AgentContext) error {
	log.Infof("%s: forwarding to pageant-pipe", PackageName)

	pipePath, err := getPageantPipePath()
	if err != nil {
		return fmt.Errorf("%s: failed to get pageant pipe path: %w", PackageName, err)
	}

	// We must hide the pageant pipe path

	dialFunction := func() (net.Conn, error) {
		conn, err := winio.DialPipe(pipePath, nil)

		if errors.Is(err, winio.ErrTimeout) {
			err = common.ErrConnectionFailedMustRetry
		}

		if err != nil {
			err = fmt.Errorf("%s: can't connect to pageant-pipe: %w", PackageName, err)
		}

		return conn, err
	}

	return common.GenericNetClient(PackageName, dialFunction, ctx)
}
