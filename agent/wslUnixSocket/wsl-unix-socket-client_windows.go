// Based on code from Copyright (c) 2017 Alexandre Bourget
// https://github.com/abourget/secrets-bridge/blob/master/pkg/agentfwd/agentconn_windows.go

package wslUnixSocket

import (
	"fmt"
	"net"

	"github.com/amurzeau/ssh-agent-bridge/agent"
	"github.com/amurzeau/ssh-agent-bridge/agent/common"
	"github.com/amurzeau/ssh-agent-bridge/log"
)

func ClientWslUnixSocket(socketPath string, ctx *agent.AgentContext) error {
	log.Infof("forwarding to WSL ssh-agent at %s", socketPath)

	dialFunction := func() (net.Conn, error) {
		conn, err := net.Dial("unix", socketPath)
		if err != nil {
			err = fmt.Errorf("%s: can't connect to %s: %w", PackageName, socketPath, err)
		}

		return conn, err
	}

	return common.GenericNetClient(PackageName, dialFunction, ctx)
}
