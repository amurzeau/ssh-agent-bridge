// Based on code from Copyright (c) 2017 Alexandre Bourget
// https://github.com/abourget/secrets-bridge/blob/master/pkg/agentfwd/agentconn_windows.go

package namedPipe

import (
	"fmt"
	"log"
	"net"

	"github.com/Microsoft/go-winio"
	"github.com/amurzeau/ssh-agent-bridge/agent"
	"github.com/amurzeau/ssh-agent-bridge/agent/common"
)

func ClientPipe(pipePath string, queryChannel chan agent.AgentMessageQuery) error {
	log.Printf("%s: forwarding to named-pipe at %s", PackageName, pipePath)

	dialFunction := func() (net.Conn, error) {
		conn, err := winio.DialPipe(pipePath, nil)

		if err == winio.ErrTimeout {
			err = common.ErrConnectionFailedMustRetry
		}

		if err != nil {
			err = fmt.Errorf("%s: can't connect to %s: %w", PackageName, pipePath, err)
		}

		return conn, err
	}

	return common.GenericNetClient(PackageName, dialFunction, queryChannel)
}
