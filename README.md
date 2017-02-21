## Simple TCP port forwarding utility

`tcpf` is a simple port forwarding utility which allows to listen to
TCP traffic on a locally bound port and redirect all the traffic to
remote TCP endpoint both ways.

Usage:
```
tcpf <local-port> <remote-host> <remote-port>
```

The tool prints the output to `stdout`.

The utility shows how the syntax of Go channels and goroutines helps
handle such tasks in a natural and elegant way.