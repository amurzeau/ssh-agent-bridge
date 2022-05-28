// Based on code from Copyright (c) 2017 Alexandre Bourget
// https://github.com/abourget/secrets-bridge/blob/master/pkg/agentfwd/agentconn_windows.go

package cygwinUnixSocket

import (
	"encoding/binary"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"os"
	"regexp"
	"time"

	"github.com/amurzeau/ssh-agent-bridge/agent"
	"github.com/amurzeau/ssh-agent-bridge/agent/common"
)

var socketRegex = regexp.MustCompile(`!<socket >(\d+) (s )?([A-Fa-f0-9-]+)`)

func connect_unix_socket(socketPath string) (net.Conn, error) {
	socketData, err := ioutil.ReadFile(socketPath)
	if err != nil {
		return nil, fmt.Errorf("%s: opening %q: %w", PackageName, socketPath, err)
	}

	matches := socketRegex.FindStringSubmatch(string(socketData))
	if matches == nil {
		return nil, fmt.Errorf("%s: bad socket file data %q: %s", PackageName, socketPath, string(socketData))
	}

	tcpPort := matches[1]
	isCygwin := matches[2] == "s "

	var guid string

	if isCygwin {
		guid = matches[3]
	} else {
		guid = matches[2]
	}

	address := fmt.Sprintf("localhost:%s", tcpPort)
	conn, err := net.Dial("tcp", address)
	if err != nil {
		return nil, fmt.Errorf("%s: can't connect to %s: %w", PackageName, address, err)
	}

	guid_raw := make([]byte, 16)
	fmt.Sscanf(guid,
		"%02x%02x%02x%02x-%02x%02x%02x%02x-%02x%02x%02x%02x-%02x%02x%02x%02x",
		&guid_raw[3], &guid_raw[2], &guid_raw[1], &guid_raw[0],
		&guid_raw[7], &guid_raw[6], &guid_raw[5], &guid_raw[4],
		&guid_raw[11], &guid_raw[10], &guid_raw[9], &guid_raw[8],
		&guid_raw[15], &guid_raw[14], &guid_raw[13], &guid_raw[12],
	)

	// fmt.Println("Writing first GUID bytes")
	if _, err = conn.Write(guid_raw); err != nil {
		return nil, fmt.Errorf("%s: write b: %w", PackageName, err)
	}

	// fmt.Println("Reading guid_reply")
	guid_reply := make([]byte, 16)
	if _, err = conn.Read(guid_reply); err != nil {
		return nil, fmt.Errorf("%s: read b2: %w", PackageName, err)
	}
	// fmt.Printf("Received b2: %q %s\n", b2, string(b2))

	// fmt.Println("Writing pid,gid,uid")
	pidsUids := make([]byte, 12)
	pid := os.Getpid()
	uid := 0
	gid := 0
	binary.LittleEndian.PutUint32(pidsUids, uint32(pid))
	binary.LittleEndian.PutUint32(pidsUids[4:], uint32(uid))
	binary.LittleEndian.PutUint32(pidsUids[8:], uint32(gid))
	// fmt.Println("  Writing", pidsUids, string(pidsUids))
	if _, err = conn.Write(pidsUids); err != nil {
		return nil, fmt.Errorf("%s: write pid,uid,gid: %w", PackageName, err)
	}

	// fmt.Println("Reading b3")
	b3 := make([]byte, 12)
	if _, err = conn.Read(b3); err != nil {
		return nil, fmt.Errorf("%s: read pid,uid,gid: %w", PackageName, err)
	}
	// fmt.Printf("Received b3: %v %s\n", b3, string(b3))

	return conn, nil
}

func ClientUnixSocket(socketPath string, queryChannel chan agent.AgentMessageQuery) error {
	log.Printf("forwarding to ssh-agent at %s", socketPath)

	dialFunction := func() (net.Conn, error) {
		conn, err := connect_unix_socket(socketPath)

		if err == os.ErrNotExist {
			time.Sleep(2 * time.Second)
			err = common.ErrConnectionFailedMustRetry
		}

		if err != nil {
			err = fmt.Errorf("%s: can't connect to %s: %w", PackageName, socketPath, err)
		}

		return conn, err
	}

	return common.GenericNetClient(PackageName, dialFunction, queryChannel)
}
