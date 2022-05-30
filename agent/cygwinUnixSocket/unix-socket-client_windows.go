// Based on code from Copyright (c) 2017 Alexandre Bourget
// https://github.com/abourget/secrets-bridge/blob/master/pkg/agentfwd/agentconn_windows.go

package cygwinUnixSocket

import (
	"encoding/binary"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"regexp"

	"github.com/amurzeau/ssh-agent-bridge/agent"
	"github.com/amurzeau/ssh-agent-bridge/agent/common"
	"github.com/amurzeau/ssh-agent-bridge/log"
)

var socketRegex = regexp.MustCompile(`!<socket >(\d+) (s )?([A-Fa-f0-9-]+)`)

func getUnixSocketInfo(socketPath string) (*string, *string, error) {
	socketData, err := ioutil.ReadFile(socketPath)
	if err != nil {
		return nil, nil, fmt.Errorf("%s: opening %q: %w", PackageName, socketPath, err)
	}

	matches := socketRegex.FindStringSubmatch(string(socketData))
	if matches == nil {
		return nil, nil, fmt.Errorf("%s: bad socket file data %q: %s", PackageName, socketPath, string(socketData))
	}

	tcpPort := matches[1]
	isCygwin := matches[2] == "s "

	var guid string

	if isCygwin {
		guid = matches[3]
	} else {
		guid = matches[2]
	}

	return &tcpPort, &guid, nil
}

func connectUnixSocket(socketPath string) (net.Conn, error) {
	log.Debugf("%s: reading socket file", PackageName)
	tcpPort, cookie, err := getUnixSocketInfo(socketPath)
	if err != nil {
		return nil, fmt.Errorf("%s: can't read socket file %s: %w", PackageName, socketPath, err)
	}

	log.Debugf("%s: connecting TCP socket", PackageName)
	address := fmt.Sprintf("127.0.0.1:%s", *tcpPort)
	conn, err := net.Dial("tcp", address)
	if err != nil {
		return nil, fmt.Errorf("%s: can't connect to %s: %w", PackageName, address, err)
	}

	log.Debugf("%s: sending cookie", PackageName)
	guid_raw := make([]byte, 16)
	fmt.Sscanf(*cookie,
		"%02x%02x%02x%02x-%02x%02x%02x%02x-%02x%02x%02x%02x-%02x%02x%02x%02x",
		&guid_raw[3], &guid_raw[2], &guid_raw[1], &guid_raw[0],
		&guid_raw[7], &guid_raw[6], &guid_raw[5], &guid_raw[4],
		&guid_raw[11], &guid_raw[10], &guid_raw[9], &guid_raw[8],
		&guid_raw[15], &guid_raw[14], &guid_raw[13], &guid_raw[12],
	)

	if _, err = conn.Write(guid_raw); err != nil {
		return nil, fmt.Errorf("%s: write b: %w", PackageName, err)
	}

	log.Debugf("%s: receiving cookie reply", PackageName)
	guid_reply := make([]byte, 16)
	if _, err = conn.Read(guid_reply); err != nil {
		return nil, fmt.Errorf("%s: read b2: %w", PackageName, err)
	}

	log.Debugf("%s: writing identification", PackageName)
	pidsUids := make([]byte, 12)
	pid := os.Getpid()
	uid := 0
	gid := 0
	binary.LittleEndian.PutUint32(pidsUids, uint32(pid))
	binary.LittleEndian.PutUint32(pidsUids[4:], uint32(uid))
	binary.LittleEndian.PutUint32(pidsUids[8:], uint32(gid))

	if _, err = conn.Write(pidsUids); err != nil {
		return nil, fmt.Errorf("%s: write pid,uid,gid: %w", PackageName, err)
	}

	log.Debugf("%s: reading identification", PackageName)
	b3 := make([]byte, 12)
	if _, err = conn.Read(b3); err != nil {
		return nil, fmt.Errorf("%s: read pid,uid,gid: %w", PackageName, err)
	}

	return conn, nil
}

func ClientUnixSocket(socketPath string, ctx *agent.AgentContext) error {
	log.Infof("forwarding to ssh-agent at %s", socketPath)

	dialFunction := func() (net.Conn, error) {
		conn, err := connectUnixSocket(socketPath)
		if err != nil {
			err = fmt.Errorf("%s: can't connect to %s: %w", PackageName, socketPath, err)
		}

		return conn, err
	}

	return common.GenericNetClient(PackageName, dialFunction, ctx)
}
