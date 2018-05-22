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
	fmt.Printf("download zip file %s %s\n", path, version)
	return fetch(mod)
}

// Query returns the directory in the local download cache
// holding the root of mod's source tree.
// It downloads the module if needed.
func Query(path string, version string) ([]module.Version, error) {
	info, err := modfetch.Query(path, version, nil)
	if err != nil {
		return nil, err
	}

	reqs := newReqs()

	mod := module.Version{Path: path, Version: info.Version}
	list, err := reqs.Required(mod)
	if err != nil {
		return nil, err
	}

	return list, nil
}
