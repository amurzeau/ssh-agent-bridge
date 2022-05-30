// Based on code from Copyright (c) 2017 Alexandre Bourget
// https://github.com/abourget/secrets-bridge/blob/master/pkg/agentfwd/agentconn_windows.go

package wslUnixSocket

import (
	"errors"
	"net"
	"os"

	"github.com/amurzeau/ssh-agent-bridge/agent"
	"github.com/amurzeau/ssh-agent-bridge/agent/common"
	"github.com/amurzeau/ssh-agent-bridge/log"
)

func ServeWslUnixSocket(socketPath string, ctx *agent.AgentContext) {
	if socketPath == "" {
		log.Errorf("%s: empty socket path, skipping serving for WSL ssh-agent queries", PackageName)
		return
	}

	result, err := os.Stat(socketPath)
	if result != nil {
		log.Errorf("%s: wsl socket path already exists: %s", PackageName, socketPath)
		return
	} else if !errors.Is(err, os.ErrNotExist) {
		log.Errorf("%s: error while checking socket path %s: %v", PackageName, socketPath, err)
		return
	}

	log.Infof("%s: listening for agent requests on WSL unix socket %v\n", PackageName, socketPath)

	// On cancel, remove the socket file
	defer os.Remove(socketPath)

	listenFunction := func() (net.Listener, error) {
		return net.Listen("unix", socketPath)
	}
	common.GenericNetServer(PackageName, listenFunction, ctx)
}
