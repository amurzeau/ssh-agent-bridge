package agent

import "encoding/binary"

type AgentMessageQuery struct {
	Data         []byte
	ReplyChannel chan AgentMessageReply
}

type AgentMessageReply struct {
	Data []byte
}

const MAX_AGENT_MESSAGE_SIZE = 262144

var AGENT_MESSAGE_ERROR_REPLY = agentMessageErrorReply()

func agentMessageErrorReply() AgentMessageReply {
	const SSH_AGENT_FAILURE = 5

	failure := make([]byte, 5)
	binary.BigEndian.PutUint32(failure, (uint32)(len(failure)-4))
	failure[4] = SSH_AGENT_FAILURE

	return AgentMessageReply{
		Data: failure,
	}
}
