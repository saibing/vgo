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
	info, err := modfetch.Query(path, version, nil)
	if err != nil {
		return "", err
	}

	reqs := newReqs()

	mod := module.Version{Path: path, Version: info.Version}
	list, err := reqs.Required(mod)
	if err != nil {
		return "", err
	}
	fmt.Println("list:", list)
	fmt.Printf("fetch module %s %s\n", path, info.Version)
	fmt.Println(info)
	return fetch(mod)
}
