// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vgo

import (
	"strings"

	"cmd/go/internal/base"
	"cmd/go/internal/load"
	"cmd/go/internal/modfetch"
	"cmd/go/internal/module"
	"cmd/go/internal/mvs"
	"cmd/go/internal/semver"
	"cmd/go/internal/work"
)

var CmdGet = &base.Command{
	UsageLine: "get [build flags] [modules or packages]",
	Short:     "download and install versioned modules and dependencies",
	Long: `
Get downloads the latest versions of modules containing the named packages,
along with the versions of the dependencies required by those modules
(not necessarily the latest ones).

It then installs the named packages, like 'go install'.

By default, get downloads the named packages, updates go.mod, and builds the packages.
As a special case if a package is a module root and has no code, no error is reported.

TODO make this better

The -m flag causes get to update the module file but not build anything.

The -d flag causes get to download the code and update the module file but not build anything.

The -u flag causes get to download the latest version of dependencies as well.

Each package being updated can be suffixed with @version to specify
the desired version. Specifying a version older than the one currently
in use causes a downgrade, which may in turn downgrade other
modules using that one, to keep everything consistent.

TODO: Make this documentation better once the semantic dust settles.
	`,
}

var (
	getD = CmdGet.Flag.Bool("d", false, "")
	getM = CmdGet.Flag.Bool("m", false, "")
	getU = CmdGet.Flag.Bool("u", false, "")
)

func init() {
	CmdGet.Run = runGet // break init loop
	work.AddBuildFlags(CmdGet)
}

func runGet(cmd *base.Command, args []string) {
	if *getU && len(args) > 0 {
		base.Fatalf("vgo get: -u not supported with argument list")
	}
	if !*getU && len(args) == 0 {
		base.Fatalf("vgo get: need arguments or -u")
	}

	if *getU {
		LoadBuildList()
		return
	}

	Init()
	InitMod()
	var upgrade []module.Version
	var downgrade []module.Version
	var newPkgs []string
	for _, pkg := range args {
		var path, vers string
		/* OLD CODE
		if n := strings.Count(pkg, "(") + strings.Count(pkg, ")"); n > 0 {
			i := strings.Index(pkg, "(")
			j := strings.Index(pkg, ")")
			if n != 2 || i < 0 || j <= i+1 || j != len(pkg)-1 && pkg[j+1] != '/' {
				base.Errorf("vgo get: invalid module version syntax: %s", pkg)
				continue
			}
			path, vers = pkg[:i], pkg[i+1:j]
			pkg = pkg[:i] + pkg[j+1:]
		*/
		if i := strings.Index(pkg, "@"); i >= 0 {
			path, pkg, vers = pkg[:i], pkg[:i], pkg[i+1:]
			if strings.Contains(vers, "@") {
				base.Errorf("vgo get: invalid module version syntax: %s", pkg)
				continue
			}
		} else {
			path = pkg
			vers = "latest"
		}
		if vers == "none" {
			downgrade = append(downgrade, module.Version{Path: path, Version: ""})
		} else {
			info, err := modfetch.Query(path, vers, allowed)
			if err != nil {
				base.Errorf("vgo get %v: %v", pkg, err)
				continue
			}
			upgrade = append(upgrade, module.Version{Path: path, Version: info.Version})
			newPkgs = append(newPkgs, pkg)
		}
	}
	args = newPkgs

	// Upgrade.
	var err error
	buildList, err = mvs.Upgrade(Target, newReqs(), upgrade...)
	if err != nil {
		base.Fatalf("vgo get: %v", err)
	}

	LoadBuildList()

	// Downgrade anything that went too far.
	version := make(map[string]string)
	for _, mod := range buildList {
		version[mod.Path] = mod.Version
	}
	for _, mod := range upgrade {
		if semver.Compare(mod.Version, version[mod.Path]) < 0 {
			downgrade = append(downgrade, mod)
		}
	}

	if len(downgrade) > 0 {
		buildList, err = mvs.Downgrade(Target, newReqs(buildList[1:]...), downgrade...)
		if err != nil {
			base.Fatalf("vgo get: %v", err)
		}

		// TODO: Check that everything we need to import is still available.
		/*
			local := v.matchPackages("all", v.Reqs[:1])
			for _, path := range local {
				dir, err := v.importDir(path)
				if err != nil {
					return err // TODO
				}
				imports, testImports, err := scanDir(dir, v.Tags)
				for _, path := range imports {
					xxx
				}
				for _, path := range testImports {
					xxx
				}
			}
		*/
	}
	WriteGoMod()

	if *getD {
		// Download all needed code as side-effect.
		ImportPaths([]string{"ALL"})
	}

	if *getM {
		return
	}

	if len(args) > 0 {
		work.BuildInit()
		var list []string
		for _, p := range load.PackagesAndErrors(args) {
			if p.Error == nil || !strings.HasPrefix(p.Error.Err, "no Go files") {
				list = append(list, p.ImportPath)
			}
		}
		if len(list) > 0 {
			work.InstallPackages(list)
		}
	}
}
