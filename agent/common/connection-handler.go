package common

import (
	"errors"
	"io"
	"net"

	"github.com/Microsoft/go-winio"
	"github.com/amurzeau/ssh-agent-bridge/agent"
	"github.com/amurzeau/ssh-agent-bridge/log"
)

var ErrConnectionFailedMustRetry = errors.New("connection failed but should be retried")

func handleClientRead(processName string, c net.Conn, ctx *agent.AgentContext, replyChannel chan agent.AgentMessageReply) {
	defer c.Close()
	defer close(replyChannel)

	doneChannel := make(chan bool)
	defer close(doneChannel)

	go func() {
		select {
		case <-ctx.Done():
			log.Debugf("%s: stopping connection", processName)
			c.Close()
		case <-doneChannel:
		}
	}()

	log.Debugf("%s: client connected [%s]", processName, c.RemoteAddr().Network())

	buf := make([]byte, 262144)
	for {
		n, err := agent.ReadAgentMessage(c, buf)
		if errors.Is(err, net.ErrClosed) {
			// intentional closing of network socket
			break
		} else if err != nil {
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
	buf := make([]byte, agent.MAX_AGENT_MESSAGE_SIZE)

	for message := range ctx.QueryChannel {
		func() { // Use an anonymous function so defer works
			conn, err := dialFunction()
			if err != nil {
				log.Errorf("%s: can't connect: %v", packageName, err)
				message.ReplyChannel <- agent.AGENT_MESSAGE_ERROR_REPLY
				return
			}

			defer conn.Close()

			_, err = conn.Write(message.Data)
			if err != nil {
				log.Errorf("%s: write failed, can't handle query, will try to reconnect: %v\n", packageName, err)
				message.ReplyChannel <- agent.AGENT_MESSAGE_ERROR_REPLY
				return
			}

			n, err := agent.ReadAgentMessage(conn, buf)
			if err != nil {
				log.Errorf("%s: reply read error, will try to reconnect: %v\n", packageName, err)
				message.ReplyChannel <- agent.AGENT_MESSAGE_ERROR_REPLY
				return
			}

			message.ReplyChannel <- agent.AgentMessageReply{Data: buf[:n]}
		}()
	}

	log.Debugf("%s: stopped", packageName)

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
		log.Debugf("%s: stopping", packageName)
		listener.Close()
	}()

	for {
		conn, err := listener.Accept()
		if errors.Is(err, net.ErrClosed) || errors.Is(err, winio.ErrPipeListenerClosed) {
			// intentional closing of network socket
			break
		} else if err != nil {
			log.Errorf("%s: accept error: %v", packageName, err)
			break
		}

		HandleAgentConnection(packageName, conn, ctx)
	}

	log.Debugf("%s: stopped", packageName)
}
