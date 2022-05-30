package main

import (
	_ "embed"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"regexp"
	"runtime"
	"strings"
	"syscall"

	"github.com/amurzeau/ssh-agent-bridge/agent"
	"github.com/amurzeau/ssh-agent-bridge/agent/cygwinUnixSocket"
	"github.com/amurzeau/ssh-agent-bridge/agent/namedPipe"
	"github.com/amurzeau/ssh-agent-bridge/agent/pageant"
	"github.com/amurzeau/ssh-agent-bridge/agent/wslUnixSocket"
	"github.com/amurzeau/ssh-agent-bridge/log"

	"github.com/getlantern/systray"
)

//go:embed assets/oxygen-status-wallet-open.ico
var assetsOxygenStatusWalletOpen []byte

var (
	argFrom                 *string
	argTo                   *string
	argPipePath             *string
	argCygwinUnixSocketPath *string
	argWslUnixSocketPath    *string

	agentContext = agent.CreateAgent()
)

var sshAgentFromMap = map[string]func(*agent.AgentContext){
	"pipe": func(ctx *agent.AgentContext) {
		namedPipe.ServePipe(*argPipePath, ctx)
	},
	"cygwin": func(ctx *agent.AgentContext) {
		cygwinUnixSocket.ServeUnixSocket(*argCygwinUnixSocketPath, ctx)
	},
	"wsl": func(ctx *agent.AgentContext) {
		wslUnixSocket.ServeWslUnixSocket(*argWslUnixSocketPath, ctx)
	},
	"pageant": func(ctx *agent.AgentContext) {
		pageant.ServePageant(ctx)
	},
}

var sshAgentToMap = map[string]func(*agent.AgentContext) error{
	"pipe": func(ctx *agent.AgentContext) error {
		return namedPipe.ClientPipe(*argPipePath, ctx)
	},
	"cygwin": func(ctx *agent.AgentContext) error {
		return cygwinUnixSocket.ClientUnixSocket(*argCygwinUnixSocketPath, ctx)
	},
	"wsl": func(ctx *agent.AgentContext) error {
		return wslUnixSocket.ClientWslUnixSocket(*argWslUnixSocketPath, ctx)
	},
	"pageant": func(ctx *agent.AgentContext) error {
		return pageant.ClientPageant(ctx)
	},
}

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

	argFrom = flag.String("from", "",
		fmt.Sprintf("comma-separated list of endpoint to listen on, available: all, %s (cygwin also work for Git for Windows)",
			strings.Join(keys(sshAgentFromMap), ", ")))

	argTo = flag.String("to", "pageant",
		fmt.Sprintf("endpoint to use as upstream agent, available: %s (cygwin also work for Git for Windows)",
			strings.Join(keys(sshAgentToMap), ", ")))

	argPipePath = flag.String("pipe", `\\.\pipe\openssh-ssh-agent`, "path to the pipe to use for pipe mode")
	argCygwinUnixSocketPath = flag.String("cygwin-socket", os.Getenv("SSH_AUTH_SOCK"), "path to the ssh-agent unix socket for cygwin-ssh-agent mode")
	argWslUnixSocketPath = flag.String("wsl-socket", os.Getenv("SSH_AUTH_SOCK"), "path to the WSL ssh-agent unix socket for wsl-ssh-agent mode")

	argDebug := flag.Bool("debug", false, "enable debug logs")
	argNoGuiError := flag.Bool("no-gui-error", false, "don't show a message box for fatal error")

	flag.Parse()

	if *argDebug {
		log.Level = log.Debug
	}

	if *argNoGuiError {
		log.UseMessageBoxForFatal = false
	}

	if *argFrom == "" {
		log.Fatalf("--from is required, see help with --help")
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

	systray.Run(onReady, onExit)
}

func onReady() {
	systray.SetIcon(assetsOxygenStatusWalletOpen)
	systray.SetTitle("SSH Agent Bridge")
	systray.SetTooltip("SSH Agent Bridge")
	mExit := systray.AddMenuItem("Exit", "Exit SSH Agent Bridge")

	go func() {
		<-mExit.ClickedCh
		agentContext.Stop()
	}()

	go func() {
		sigs := make(chan os.Signal, 1)
		signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
		<-sigs
		agentContext.Stop()
	}()

	go func() {
		<-agentContext.Done()
		agentContext.Wait()
		systray.Quit()
	}()

	// Listen on all requested endpoints
	fromValues := strings.Split(strings.ReplaceAll(*argFrom, " ", ""), ",")

	for _, from := range fromValues {
		if serverHandler, ok := sshAgentFromMap[from]; ok {
			log.Infof("Handling ssh agent queries from %s", from)

			agentContext.Go(func() {
				serverHandler(&agentContext)
			})
		} else {
			log.Fatalf("Bad --from value %s, available: %s",
				from,
				strings.Join(keys(sshAgentFromMap), ", "))
			agentContext.Stop()
		}
	}

	if clientHandler, ok := sshAgentToMap[*argTo]; ok {
		log.Infof("Forwarding ssh agent queries to %s", *argTo)
		// Run upstream agent handler
		agentContext.Go(func() {
			err := clientHandler(&agentContext)
			if err != nil {
				log.Fatalf("error with upstream agent: %v", err)
				agentContext.Stop()
			}
		})
	} else {
		log.Fatalf("Bad --to value %s, available: %s",
			*argTo,
			strings.Join(keys(sshAgentToMap), ", "))
		agentContext.Stop()
	}
}

func onExit() {
	os.Exit(0)
}
