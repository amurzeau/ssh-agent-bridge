package main

import (
	"flag"
	"fmt"
	"os"
	"regexp"
	"runtime"
	"strings"

	"github.com/amurzeau/ssh-agent-bridge/agent"
	"github.com/amurzeau/ssh-agent-bridge/agent/cygwinUnixSocket"
	"github.com/amurzeau/ssh-agent-bridge/agent/namedPipe"
	"github.com/amurzeau/ssh-agent-bridge/agent/pageant"
	"github.com/amurzeau/ssh-agent-bridge/agent/wslUnixSocket"
	"github.com/amurzeau/ssh-agent-bridge/log"
)

func keys[T any, Key comparable](m map[Key]T) []Key {
	j := 0
	keys := make([]Key, len(m))
	for k := range m {
		keys[j] = k
		j++
	}
	return keys
}

func remove[T comparable](l []T, item T) []T {
	for i, other := range l {
		if other == item {
			return append(l[:i], l[i+1:]...)
		}
	}
	return l
}

var reCygwinTmpDir = regexp.MustCompile(`^/tmp`)
var reCygwinDriveDir = regexp.MustCompile(`^(/cygdrive)?/([a-z])/`)

func convertCygwinPathToWindows(path string) string {
	tmpPath := os.TempDir()

	nativePath := path
	nativePath = reCygwinTmpDir.ReplaceAllLiteralString(nativePath, tmpPath)
	nativePath = reCygwinDriveDir.ReplaceAllString(nativePath, "$2:/")

	if path != nativePath {
		log.Debugf("converting cygwin path from %s to %s",
			path,
			nativePath)
	}

	return nativePath
}

func main() {
	var (
		argFrom                 *string
		argTo                   *string
		argPipePath             *string
		argCygwinUnixSocketPath *string
		argWslUnixSocketPath    *string
	)

	var sshAgentFromMap = map[string]func(chan agent.AgentMessageQuery){
		"pipe": func(queryChannel chan agent.AgentMessageQuery) {
			namedPipe.ServePipe(*argPipePath, queryChannel)
		},
		"cygwin-ssh-agent": func(queryChannel chan agent.AgentMessageQuery) {
			cygwinUnixSocket.ServeUnixSocket(*argCygwinUnixSocketPath, queryChannel)
		},
		"wsl-ssh-agent": func(queryChannel chan agent.AgentMessageQuery) {
			wslUnixSocket.ServeWslUnixSocket(*argWslUnixSocketPath, queryChannel)
		},
		"pageant": func(queryChannel chan agent.AgentMessageQuery) {
			pageant.ServePageant(queryChannel)
		},
	}

	var sshAgentToMap = map[string]func(chan agent.AgentMessageQuery) error{
		"pipe": func(queryChannel chan agent.AgentMessageQuery) error {
			return namedPipe.ClientPipe(*argPipePath, queryChannel)
		},
		"cygwin-ssh-agent": func(queryChannel chan agent.AgentMessageQuery) error {
			return cygwinUnixSocket.ClientUnixSocket(*argCygwinUnixSocketPath, queryChannel)
		},
		"wsl-ssh-agent": func(queryChannel chan agent.AgentMessageQuery) error {
			return wslUnixSocket.ClientWslUnixSocket(*argWslUnixSocketPath, queryChannel)
		},
		"pageant": func(queryChannel chan agent.AgentMessageQuery) error {
			return pageant.ClientPageant(queryChannel)
		},
	}

	argFrom = flag.String("from", "all",
		fmt.Sprintf("comma-separated list of endpoint to listen on, available: all, %s",
			strings.Join(keys(sshAgentFromMap), ", ")))

	argTo = flag.String("to", "pageant",
		fmt.Sprintf("endpoint to use as upstream agent, available: %s",
			strings.Join(keys(sshAgentToMap), ", ")))

	argPipePath = flag.String("pipe", `\\.\pipe\openssh-ssh-agent`, "path to the pipe to use for pipe mode")
	argCygwinUnixSocketPath = flag.String("unix-socket", os.Getenv("SSH_AUTH_SOCK"), "path to the ssh-agent unix socket for cygwin-ssh-agent mode")
	argWslUnixSocketPath = flag.String("wsl-socket", os.Getenv("SSH_AUTH_SOCK"), "path to the WSL ssh-agent unix socket for wsl-ssh-agent mode")

	argDebug := flag.Bool("debug", false, "enable debug logs")

	flag.Parse()

	if *argDebug {
		log.Level = log.Debug
	}

	// By default, listen on every possible supported endpoint except the one used as upstream agent
	if *argFrom == "all" {
		fromKeys := keys(sshAgentFromMap)
		fromKeys = remove(fromKeys, *argTo)
		*argFrom = strings.Join(fromKeys, ",")
	}

	// Convert cygwin/msys paths to native Windows path
	if runtime.GOOS == "windows" {
		*argCygwinUnixSocketPath = convertCygwinPathToWindows(*argCygwinUnixSocketPath)
		*argWslUnixSocketPath = convertCygwinPathToWindows(*argWslUnixSocketPath)
	}

	// A single channel is used for all queries, as we support only one target "to"
	queryChannel := make(chan agent.AgentMessageQuery)

	// Listen on all requested endpoints
	fromValues := strings.Split(strings.ReplaceAll(*argFrom, " ", ""), ",")

	for _, from := range fromValues {
		if serverHandler, ok := sshAgentFromMap[from]; ok {
			log.Infof("Handling ssh agent queries from %s", from)

			go serverHandler(queryChannel)
		} else {
			log.Fatalf("Bad --from value %s, available: %s",
				from,
				strings.Join(keys(sshAgentFromMap), ", "))
		}
	}

	if clientHandler, ok := sshAgentToMap[*argTo]; ok {
		log.Infof("Forwarding ssh agent queries to %s", *argTo)
		// Run upstream agent handler (blocking)
		err := clientHandler(queryChannel)
		if err != nil {
			log.Fatalf("error with upstream agent: %v", err)
		}
	} else {
		log.Fatalf("Bad --to value %s, available: %s",
			*argTo,
			strings.Join(keys(sshAgentToMap), ", "))
	}
}
