// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vgo

import (
	"bytes"
	"cmd/go/internal/base"
	"cmd/go/internal/cfg"
	"cmd/go/internal/modfetch"
	"cmd/go/internal/modinfo"
	"cmd/go/internal/module"
	"cmd/go/internal/search"
	"cmd/go/internal/semver"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var (
	infoStart, _ = hex.DecodeString("3077af0c9274080241e1c107e6d618e6")
	infoEnd, _   = hex.DecodeString("f932433186182072008242104116d8f2")
)

func isStandardImportPath(path string) bool {
	if search.IsStandardImportPath(path) {
		if _, err := os.Stat(filepath.Join(cfg.GOROOT, "src", path)); err == nil {
			return true
		}
		if _, err := os.Stat(filepath.Join(cfg.GOROOT, "src/vendor", path)); err == nil {
			return true
		}
	}
	return false
}

func PackageModuleInfo(pkgpath string) *modinfo.ModulePublic {
	if isStandardImportPath(pkgpath) || !Enabled() {
		return nil
	}
	return moduleInfo(findModule(pkgpath, pkgpath))
}

func ModuleInfo(path string) *modinfo.ModulePublic {
	if !Enabled() {
		return nil
	}

	if i := strings.Index(path, "@"); i >= 0 {
		return moduleInfo(module.Version{Path: path[:i], Version: path[i+1:]})
	}

	for _, m := range buildList {
		if m.Path == path {
			return moduleInfo(m)
		}
	}

	return &modinfo.ModulePublic{
		Path: path,
		Error: &modinfo.ModuleError{
			Err: "module not in current build",
		},
	}
}

// AddUpdate fills in m.Update if an updated version is available.
func AddUpdate(m *modinfo.ModulePublic) {
	addUpdate(m)
	if m.Replace != nil {
		addUpdate(m.Replace)
	}
}

func addUpdate(m *modinfo.ModulePublic) {
	if m.Version != "" {
		if info, err := modfetch.Query(m.Path, "latest", allowed); err == nil && info.Version != m.Version {
			m.Update = &modinfo.ModulePublic{
				Path:    m.Path,
				Version: info.Version,
				Time:    &info.Time,
			}
		}
	}
}

func moduleInfo(m module.Version) *modinfo.ModulePublic {
	if m == Target {
		return &modinfo.ModulePublic{
			Path:    m.Path,
			Version: m.Version,
			Main:    true,
		}
	}

	info := &modinfo.ModulePublic{
		Path:    m.Path,
		Version: m.Version,
	}

	// complete fills in the extra fields in m.
	complete := func(m *modinfo.ModulePublic) {
		if m.Version != "" {
			if q, err := modfetch.Query(m.Path, m.Version, nil); err != nil {
				m.Error = &modinfo.ModuleError{Err: err.Error()}
			} else {
				m.Version = q.Version
				m.Time = &q.Time
			}

			if semver.IsValid(m.Version) {
				dir := filepath.Join(SrcMod, m.Path+"@"+m.Version)
				if stat, err := os.Stat(dir); err == nil && stat.IsDir() {
					m.Dir = dir
				}
			}
		}
		if cfg.BuildGetmode == "vendor" {
			m.Dir = filepath.Join(ModRoot, "vendor", m.Path)
		}
	}

	complete(info)

	if r := Replacement(m); r.Path != "" {
		info.Replace = &modinfo.ModulePublic{
			Path:    r.Path,
			Version: r.Version,
		}
		if r.Version == "" {
			if filepath.IsAbs(r.Path) {
				info.Replace.Dir = r.Path
			} else {
				info.Replace.Dir = filepath.Join(ModRoot, r.Path)
			}
		}
		complete(info.Replace)
		info.Dir = info.Replace.Dir
		info.Error = nil // ignore error loading original module version (it has been replaced)
	}

	return info
}

func PackageBuildInfo(path string, deps []string) string {
	if isStandardImportPath(path) || !Enabled() {
		return ""
	}
	target := findModule(path, path)
	mdeps := make(map[module.Version]bool)
	for _, dep := range deps {
		if !isStandardImportPath(dep) {
			mdeps[findModule(path, dep)] = true
		}
	}
	var mods []module.Version
	delete(mdeps, target)
	for mod := range mdeps {
		mods = append(mods, mod)
	}
	module.Sort(mods)

	var buf bytes.Buffer
	fmt.Fprintf(&buf, "path\t%s\n", path)
	tv := target.Version
	if tv == "" {
		tv = "(devel)"
	}
	fmt.Fprintf(&buf, "mod\t%s\t%s\t%s\n", target.Path, tv, modfetch.Sum(target))
	for _, mod := range mods {
		mv := mod.Version
		if mv == "" {
			mv = "(devel)"
		}
		r := Replacement(mod)
		h := ""
		if r.Path == "" {
			h = "\t" + modfetch.Sum(mod)
		}
		fmt.Fprintf(&buf, "dep\t%s\t%s%s\n", mod.Path, mod.Version, h)
		if r.Path != "" {
			fmt.Fprintf(&buf, "=>\t%s\t%s\t%s\n", r.Path, r.Version, modfetch.Sum(r))
		}
	}
	return buf.String()
}

func findModule(target, path string) module.Version {
	if path == "." {
		return buildList[0]
	}
	for _, mod := range buildList {
		if importPathInModule(path, mod.Path) {
			return mod
		}
	}
	base.Fatalf("build %v: cannot find module for path %v", target, path)
	panic("unreachable")
}

func ModInfoProg(info string) []byte {
	return []byte(fmt.Sprintf(`
		package main
		import _ "unsafe"
		//go:linkname __debug_modinfo__ runtime/debug.modinfo
		var __debug_modinfo__ string
		func init() {
			__debug_modinfo__ = %q
		}
	`, string(infoStart)+info+string(infoEnd)))
}
