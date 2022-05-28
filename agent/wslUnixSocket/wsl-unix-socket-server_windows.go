// Based on code from Copyright (c) 2017 Alexandre Bourget
// https://github.com/abourget/secrets-bridge/blob/master/pkg/agentfwd/agentconn_windows.go

package wslUnixSocket

import (
	"net"

	"github.com/amurzeau/ssh-agent-bridge/agent"
	"github.com/amurzeau/ssh-agent-bridge/agent/common"
	"github.com/amurzeau/ssh-agent-bridge/log"
)

func ServeWslUnixSocket(socketPath string, queryChannel chan agent.AgentMessageQuery) {
	if socketPath == "" {
		log.Errorf("%s: empty socket path, skipping serving for WSL ssh-agent queries", PackageName)
		return
	}

	log.Infof("%s: listening for agent requests on WSL unix socket %v\n", PackageName, socketPath)

	listenFunction := func() (net.Listener, error) {
		return net.Listen("unix", socketPath)
	}
	common.GenericNetServer(PackageName, listenFunction, queryChannel)
}
