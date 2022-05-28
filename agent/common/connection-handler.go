package common

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net"

	"github.com/amurzeau/ssh-agent-bridge/agent"
)

var ErrConnectionFailedMustRetry = errors.New("connection failed but should be retried")

func handleClientRead(processName string, c net.Conn, queryChannel chan agent.AgentMessageQuery, replyChannel chan agent.AgentMessageReply) {
	defer c.Close()
	defer close(replyChannel)

	log.Printf("%s: client connected [%s]", processName, c.RemoteAddr().Network())

	buf := make([]byte, 262144)
	for {
		n, err := agent.ReadAgentMessage(c, buf)
		if err != nil {
			if err != io.EOF {
				log.Printf("%s: read error: %v\n", processName, err)
			}
			break
		}

		message := agent.AgentMessageQuery{Data: buf[:n], ReplyChannel: replyChannel}

		log.Printf("%s: read %d data\n", processName, len(message.Data))

		queryChannel <- message
	}
	log.Printf("%s: client disconnected", processName)
}

func handleClientWrite(processName string, c net.Conn, replyChannel chan agent.AgentMessageReply) {
	defer c.Close()

	for {
		message, more := <-replyChannel

		if !more {
			break
		}

		log.Printf("%s: write %d data\n", processName, len(message.Data))

		_, err := c.Write(message.Data)
		if err != nil {
			if err != io.EOF {
				log.Printf("%s: write error: %v\n", processName, err)
			}
			break
		}
	}
}

func HandleAgentConnection(processName string, conn net.Conn, queryChannel chan agent.AgentMessageQuery) {
	replyChannel := make(chan agent.AgentMessageReply)

	go handleClientRead(processName, conn, queryChannel, replyChannel)
	go handleClientWrite(processName, conn, replyChannel)
}

func GenericNetClient(packageName string, dialFunction func() (net.Conn, error), queryChannel chan agent.AgentMessageQuery) error {
	var retryConnection bool = true

	// Try to connect indefinitely to support reconnection (or restart of the agent server)
	var lastConnectionSucceeded = true // Used for logging
	for retryConnection {
		conn, err := dialFunction()
		if err == ErrConnectionFailedMustRetry {
			if lastConnectionSucceeded {
				lastConnectionSucceeded = false
				log.Printf("%s: failed to connect (successive failures won't be logged): %v", packageName, err)
			}
			// Retry to connect
			continue
		} else if err != nil {
			return fmt.Errorf("%s: can't connect: %w", packageName, err)
		}
		defer conn.Close()

		lastConnectionSucceeded = true

		buf := make([]byte, agent.MAX_AGENT_MESSAGE_SIZE)

		// If we go out of the following for, queryChannel was closed and we should not try to reconnect
		retryConnection = false

		for message := range queryChannel {
			_, err := conn.Write(message.Data)
			if err != nil {
				log.Printf("%s: write failed, can't handle query, will try to reconnect: %v\n", packageName, err)
				// Requeue the message
				queryChannel <- message
				break
			}

			n, err := agent.ReadAgentMessage(conn, buf)
			if err != nil {
				log.Printf("%s: reply read error, will try to reconnect: %v\n", packageName, err)
				// Requeue the message
				queryChannel <- message
				break
			}

			message.ReplyChannel <- agent.AgentMessageReply{Data: buf[:n]}
		}
	}

	return nil
}

func GenericNetServer(packageName string, listenFunction func() (net.Listener, error), queryChannel chan agent.AgentMessageQuery) {
	listener, err := listenFunction()
	if err != nil {
		log.Printf("%s: listen error: %v", packageName, err)
		return
	}
	defer listener.Close()

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("%s: accept error: %v", packageName, err)
			return
		}

		HandleAgentConnection(packageName, conn, queryChannel)
	}
}
