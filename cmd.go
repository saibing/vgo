package main

import (
	"flag"
	"fmt"
)

const (
	proxyVersion = "vgoproxy 1.0.0"
)

// Cmd the bingo command line arguments.
type Cmd struct {
	HelpFlag    bool
	VersionFlag bool
	IP          string
	Port        string
	Config      string
	Args        []string
}

// parseCmd parse the bingo command line arguments.
func parseCmd() *Cmd {
	cmd := &Cmd{}

	flag.Usage = printUsage
	flag.BoolVar(&cmd.HelpFlag, "help", false, "print help message")
	flag.BoolVar(&cmd.VersionFlag, "version", false, "print version and exit")
	flag.StringVar(&cmd.IP, "ip", "", "the listen ip address")
	flag.StringVar(&cmd.Port, "port", "9090", "the listen port")
	flag.StringVar(&cmd.Config, "config", "./vgo.json", "the vgo proxy config file")

	flag.Parse()

	cmd.Args = flag.Args()

	return cmd
}

// printUsage print bingo usage
func printUsage() {
	fmt.Println("Usage: vgo --ip <ip> --port <port> --config <config file path>")
}

// printVersion print bingo version
func printVersion() {
	fmt.Println(proxyVersion)
}
