# ssh-agent-bridge

This tool can bridge multiple ssh-agent protocols to one ssh-agent.
Supported protocols are:

- pageant
- Git for Windows ssh-agent
- OpenSSH Win32 ssh-agent (Windows pipe)
- WSL ssh-agent socket using Windows' AF_UNIX sockets

This tool can listen for any of these and forward agent queries to any of these too.

# Usage

Run `./ssh-agent-bridge.exe --help`:
```
Usage of D:\Projets_C-build\ssh-agent-bridge.exe:
  -debug
        enable debug logs
  -from string
        comma-separated list of endpoint to listen on, available: all, pipe, cygwin, wsl, pageant (cygwin also work for Git for Windows)
  -no-gui-error
        don't show a message box for fatal error
  -pipe string
        path to the pipe to use for pipe mode (default "\\.\pipe\openssh-ssh-agent")
  -to string
        endpoint to use as upstream agent, available: cygwin, wsl, pageant, pipe (cygwin also work for Git for Windows) (default "pageant")
  -cygwin-socket string
        path to the ssh-agent unix socket for cygwin-ssh-agent mode (default to SSH_AUTH_SOCK env variable)
  -wsl-socket string
        path to the WSL ssh-agent unix socket for wsl-ssh-agent mode (defaults to SSH_AUTH_SOCK env variable)
```

## Usage example

Forwarding requests to `pageant`, from all of:

- Cygwin/Git for Windows on `/tmp/ssh-0PcrJrq8KjAL/agent.418` (the cygwin path /tmp will be converted internally to `%TMP%`)
- Win32 OpenSSH on `\\.\pipe\openssh-ssh-agent` (the default)
- WSL socket on `C:\wsl-ssh-agent.sock`

Command line:
```sh
./ssh-agent-bridge.exe \
  --from cygwin,pipe,wsl \
  --to pageant \
  --cygwin-socket C:/git-bash-ssh-agent.sock \
  --wsl-socket C:/wsl-ssh-agent.sock
```

Then:

- In git bash, set `export SSH_AUTH_SOCK=/c/git-bash-ssh-agent.sock`
- In WSL, set `export SSH_AUTH_SOCK=/mnt/c/wsl-ssh-agent.sock`
