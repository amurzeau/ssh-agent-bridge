// Based on code from Copyright (c) 2017 Alexandre Bourget
// https://github.com/abourget/secrets-bridge/blob/master/pkg/agentfwd/agentconn_windows.go

package wslUnixSocket

import (
	"errors"
	"fmt"
	"net"
	"os"
	"time"

	"github.com/amurzeau/ssh-agent-bridge/agent"
	"github.com/amurzeau/ssh-agent-bridge/agent/common"
	"github.com/amurzeau/ssh-agent-bridge/log"
)

func ClientWslUnixSocket(socketPath string, ctx *agent.AgentContext) error {
	log.Infof("forwarding to WSL ssh-agent at %s", socketPath)

	dialFunction := func() (net.Conn, error) {
		conn, err := net.Dial("unix", socketPath)

		if errors.Is(err, os.ErrNotExist) {
			log.Debugf("%s: sleeping 2s", PackageName)
			time.Sleep(2 * time.Second)
			err = common.ErrConnectionFailedMustRetry
		}

		if err != nil {
			err = fmt.Errorf("%s: can't connect to %s: %w", PackageName, socketPath, err)
		}

		return conn, err
	}

	return common.GenericNetClient(PackageName, dialFunction, ctx)
}
