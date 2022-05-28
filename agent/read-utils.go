package agent

import (
	"encoding/binary"
	"errors"
	"net"
)

type ReadFunction func(b []byte) (n int, err error)

var ErrBufferTooSmall = errors.New("can't read agent reply, buffer too small")

func ReadAgentMessage(conn net.Conn, buf []byte) (n int, err error) {
	var bytesRead = 0
	var messageSize = len(buf)
	var messageSizeParsed = false

	for bytesRead < messageSize {
		n, err := conn.Read(buf[bytesRead:messageSize])
		if err != nil {
			return 0, err
		}

		bytesRead += n

		// Update remainingByteToRead if enough bytes are received to read the message length
		if bytesRead >= 4 && !messageSizeParsed {
			// Add 4 bytes for the length field itself
			messageSize = int(binary.BigEndian.Uint32(buf[0:4])) + 4
			if messageSize > len(buf) {
				return 0, ErrBufferTooSmall
			}
			messageSizeParsed = true
		}
	}

	return bytesRead, nil
}
