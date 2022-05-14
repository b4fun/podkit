package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"

	bflag "github.com/b4fun/battery/flag"
	"github.com/b4fun/podkit"
	"github.com/b4fun/podkit/examples"
	"github.com/b4fun/podkit/podforward"
)

var (
	flagNamespace     string
	flagLabelSelector string
	flagKubeConfig    *string
	flagPorts         bflag.RepeatedStringSlice
)

func setupFlags() {
	flagKubeConfig = examples.BindCLIFlags(flag.CommandLine)
	flag.StringVar(&flagNamespace, "namespace", "", "Specify the namespace to use.")
	flag.StringVar(&flagLabelSelector, "selector", "", "Selector (label query) to filter on.")
	flag.Var(&flagPorts, "port", "Forward local port to remote port. Format: [localPort]:[remotePort].")

	flag.Parse()

	if flagNamespace == "" {
		flagNamespace = "default"
	}

	if flagLabelSelector == "" {
		panic("label selector is required")
	}
}

func mustParsePort(port string) uint16 {
	p, err := strconv.ParseUint(port, 10, 16)
	if err != nil {
		panic(err)
	}
	return uint16(p)
}

func main() {
	setupFlags()

	restConfig, err := examples.OutOfClusterRestConfig(*flagKubeConfig)
	if err != nil {
		panic(err.Error())
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	options := []podforward.Option{
		podforward.WithLogger(podkit.LogFunc(func(format string, args ...interface{}) {
			fmt.Printf(format+"\n", args...)
		})),
		podforward.FromSelectedPods(flagLabelSelector),
	}
	for _, portPair := range flagPorts {
		var localPort, remotePort uint16
		if strings.Contains(portPair, ":") {
			parts := strings.SplitN(portPair, ":", 2)
			if parts[0] == "" {
				localPort = 0
			} else {
				localPort = mustParsePort(parts[0])
			}
			if parts[1] == "" {
				panic(fmt.Sprintf("invalid input: %q", portPair))
			}
			remotePort = mustParsePort(parts[1])
		} else {
			remotePort = mustParsePort(portPair)
			localPort = remotePort
		}

		options = append(options, podforward.FromRemotePort(remotePort).ToLocalPort(localPort))
	}

	pf, err := podforward.Forward(ctx, restConfig, flagNamespace, options...)
	if err != nil {
		panic(err.Error())
	}
	defer pf.StopForward()

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt)
	<-sigs
}
