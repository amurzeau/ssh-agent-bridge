// Based on code from Copyright (c) 2017 Alexandre Bourget
// https://github.com/abourget/secrets-bridge/blob/master/pkg/agentfwd/agentconn_windows.go

package cygwinUnixSocket

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"os"

	"github.com/amurzeau/ssh-agent-bridge/agent"
	"github.com/amurzeau/ssh-agent-bridge/agent/common"
	"github.com/amurzeau/ssh-agent-bridge/log"
)

func handshakeConnection(conn net.Conn, expectedCookie []byte) error {
	cookie := make([]byte, 16)
	_, err := io.ReadFull(conn, cookie)
	if err != nil {
		return fmt.Errorf("%s: couldn't read cookie: %w", PackageName, err)
	}

	if !bytes.Equal(cookie, expectedCookie) {
		return fmt.Errorf("%s: invalid cookie,\n"+
			"received: %s\n"+
			"expected: %s",
			PackageName,
			hex.EncodeToString(cookie),
			hex.EncodeToString(expectedCookie))
	}

	// Send back the 16 bytes cookie
	conn.Write(cookie)

	identificationData := make([]byte, 12)
	_, err = io.ReadFull(conn, identificationData)
	if err != nil {
		return fmt.Errorf("%s: couldn't read identification data: %w", PackageName, err)
	}

	// Send back identification data
	pidsUids := make([]byte, 12)
	pid := os.Getpid()
	binary.LittleEndian.PutUint32(pidsUids, uint32(pid))
	binary.LittleEndian.PutUint32(pidsUids[4:], 0)
	binary.LittleEndian.PutUint32(pidsUids[8:], 0)
	conn.Write(pidsUids)

	return nil
}

func ServeUnixSocket(socketPath string, queryChannel chan agent.AgentMessageQuery) {
	if socketPath == "" {
		log.Errorf("%s: empty socket path, skipping serving for ssh-agent queries", PackageName)
		return
	}

	log.Infof("%s: listening for ssh-agent requests on %s", PackageName, socketPath)

	// Use 0 as the port to listen on a random available port
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		log.Errorf("%s: failed to listen a TCP port: %v", PackageName, err)
		return
	}
	defer listener.Close()

	cookie := make([]byte, 16)
	_, err = rand.Read(cookie)
	if err != nil {
		log.Errorf("%s: failed to generate a random cookie: %v", PackageName, err)
		return
	}

	socketData := fmt.Sprintf("!<socket >%d s %02x%02x%02x%02x-%02x%02x%02x%02x-%02x%02x%02x%02x-%02x%02x%02x%02x",
		listener.Addr().(*net.TCPAddr).Port,
		cookie[3], cookie[2], cookie[1], cookie[0],
		cookie[7], cookie[6], cookie[5], cookie[4],
		cookie[11], cookie[10], cookie[9], cookie[8],
		cookie[15], cookie[14], cookie[13], cookie[12],
	)

	err = ioutil.WriteFile(socketPath, []byte(socketData), 0777)
	if err != nil {
		log.Errorf("%s: failed to write file %s: %v", PackageName, socketPath, err)
		return
	}

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Errorf("%s: accept error: %v", PackageName, err)
			return
		}

		err = handshakeConnection(conn, cookie)
		if err != nil {
			log.Errorf("%s: handshake failed: %v", PackageName, err)
			conn.Close()
			continue
		}

		common.HandleAgentConnection(PackageName, conn, queryChannel)
	}
}
