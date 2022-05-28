// Based on code from Copyright (c) 2017 Alexandre Bourget
// https://github.com/abourget/secrets-bridge/blob/master/pkg/agentfwd/agentconn_windows.go

package wslUnixSocket

import (
	"log"
	"net"

	"github.com/amurzeau/ssh-agent-bridge/agent"
	"github.com/amurzeau/ssh-agent-bridge/agent/common"
)

func ServeWslUnixSocket(socketPath string, queryChannel chan agent.AgentMessageQuery) {
	if socketPath == "" {
		log.Printf("%s: empty socket path, skipping serving for WSL ssh-agent queries", PackageName)
		return
	}

	log.Printf("%s: listening for agent requests on WSL unix socket %v\n", PackageName, socketPath)

	listenFunction := func() (net.Listener, error) {
		return net.Listen("unix", socketPath)
	}
	common.GenericNetServer(PackageName, listenFunction, queryChannel)
}
