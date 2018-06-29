// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Vgo is a prototype of what the go command
// might look like with integrated support for package versioning.
//
// Download and install with:
//
//	go get -u golang.org/x/vgo
//
// Then run "vgo" instead of "go".
//
// See https://research.swtch.com/vgo-intro for an overview
// and the documents linked at https://research.swtch.com/vgo
// for additional details.
//
// This is still a very early prototype.
// You are likely to run into bugs.
// Please file bugs in the main Go issue tracker,
// https://golang.org/issue,
// and put the prefix `x/vgo:` in the issue title.
//
// Thank you.
//
package main

import (
	"cmd/go/proxy"
	"fmt"
	"os"

	_ "net/http/pprof"
	"log"
	"net/http"
	"io/ioutil"
	"encoding/json"
)

func main() {
	cmd := parseCmd()
	if cmd.HelpFlag {
		printUsage()
		return
	}

	if cmd.VersionFlag {
		printVersion()
		return
	}

	if cmd.Config == "" {
		printUsage()
		return
	}

	data, err := ioutil.ReadFile(cmd.Config)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return
	}

	cfg := &proxy.Config{}

	err = json.Unmarshal(data, cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return
	}

	cfg.Init()

	go func() {
		log.Println(http.ListenAndServe("localhost:6060", nil))
	}()

	proxy.Serve(cmd.IP, cmd.Port, cfg)
}
