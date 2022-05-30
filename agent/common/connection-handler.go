package common

import (
	"errors"
	"fmt"
	"io"
	"net"

	"github.com/amurzeau/ssh-agent-bridge/agent"
	"github.com/amurzeau/ssh-agent-bridge/log"
)

var ErrConnectionFailedMustRetry = errors.New("connection failed but should be retried")

func handleClientRead(processName string, c net.Conn, ctx *agent.AgentContext, replyChannel chan agent.AgentMessageReply) {
	defer c.Close()
	defer close(replyChannel)

	go func() {
		<-ctx.Done()
		c.Close()
	}()

	log.Debugf("%s: client connected [%s]", processName, c.RemoteAddr().Network())

	buf := make([]byte, 262144)
	for {
		n, err := agent.ReadAgentMessage(c, buf)
		if err != nil {
			if err != io.EOF {
				log.Debugf("%s: read error: %v\n", processName, err)
			}
			break
		}

		message := agent.AgentMessageQuery{Data: buf[:n], ReplyChannel: replyChannel}

		log.Debugf("%s: read %d data\n", processName, len(message.Data))

		ctx.QueryChannel <- message
	}
	log.Debugf("%s: client disconnected", processName)
}

func handleClientWrite(processName string, c net.Conn, replyChannel chan agent.AgentMessageReply) {
	for message := range replyChannel {
		log.Debugf("%s: write %d data\n", processName, len(message.Data))

		_, err := c.Write(message.Data)
		if err != nil {
			if err != io.EOF {
				log.Debugf("%s: write error: %v\n", processName, err)
			}
			break
		}
	}
}

func HandleAgentConnection(processName string, conn net.Conn, ctx *agent.AgentContext) {
	replyChannel := make(chan agent.AgentMessageReply)

	ctx.Go(func() {
		handleClientRead(processName, conn, ctx, replyChannel)
	})
	ctx.Go(func() {
		handleClientWrite(processName, conn, replyChannel)
	})
}

func GenericNetClient(packageName string, dialFunction func() (net.Conn, error), ctx *agent.AgentContext) error {
	var retryConnection bool = true

	// Try to connect indefinitely to support reconnection (or restart of the agent server)
	var lastConnectionSucceeded = true // Used for logging
	for retryConnection {
		conn, err := dialFunction()
		if errors.Is(err, ErrConnectionFailedMustRetry) {
			if lastConnectionSucceeded {
				lastConnectionSucceeded = false
				log.Errorf("%s: failed to connect (successive failures won't be logged): %v", packageName, err)
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

		for message := range ctx.QueryChannel {
			_, err := conn.Write(message.Data)
			if err != nil {
				log.Errorf("%s: write failed, can't handle query, will try to reconnect: %v\n", packageName, err)
				// Requeue the message
				ctx.QueryChannel <- message
				break
			}

			n, err := agent.ReadAgentMessage(conn, buf)
			if err != nil {
				log.Errorf("%s: reply read error, will try to reconnect: %v\n", packageName, err)
				// Requeue the message
				ctx.QueryChannel <- message
				break
			}

			message.ReplyChannel <- agent.AgentMessageReply{Data: buf[:n]}
		}
	}

	return nil
}

func GenericNetServer(packageName string, listenFunction func() (net.Listener, error), ctx *agent.AgentContext) {
	listener, err := listenFunction()
	if err != nil {
		log.Errorf("%s: listen error: %v", packageName, err)
		return
	}
	defer listener.Close()

	go func() {
		<-ctx.Done()
		listener.Close()
	}()

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Errorf("%s: accept error: %v", packageName, err)
			return
		}

		HandleAgentConnection(packageName, conn, ctx)
	}
}
