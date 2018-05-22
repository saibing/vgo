package vgo

import (
	"fmt"

	"cmd/go/internal/modfetch"
	"cmd/go/internal/module"
)

// Fetch returns the directory in the local download cache
// holding the root of mod's source tree.
// It downloads the module if needed.
func Fetch(path string, version string) (dir string, err error) {
	mod := module.Version{Path: path, Version: version}
	fmt.Printf("vgo: download zip file %s %s\n", path, version)
	return fetch(mod)
}

func Versions(path string) ([]string, error) {
	return versions(path)
}

// Query returns the directory in the local download cache
// holding the root of mod's source tree.
// It downloads the module if needed.
func Query(path string, version string) ([]module.Version, error) {
	info, err := modfetch.Query(path, version, nil)
	if err != nil {
		return nil, err
	}
	fmt.Printf("vgo: module info: %v\n", info)
	return required(path, info.Version)
}

func required(path string, version string) ([]module.Version, error) {
	reqs := newReqs()
	mod := module.Version{Path: path, Version: version}
	list, err := reqs.Required(mod)
	if err != nil {
		return nil, err
	}

	return list, nil
}

func Module(path string, version string) (*modfetch.RevInfo, error) {
	return modfetch.Query(path, version, nil)
}
