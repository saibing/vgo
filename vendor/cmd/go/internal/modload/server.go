package modload

import (
	"cmd/go/internal/module"
	"cmd/go/internal/modfetch"
)

// holding the root of mod's source tree.
// It downloads the module if needed.
func ServerFetch(path string, version string) (string, bool, error) {
	mod := module.Version{Path: path, Version: version}
	return fetch(mod)
}

func ServerModule(path string, version string) (*modfetch.RevInfo, error) {
	return Query(path, version, nil)
}

func ServerVersions(path string) ([]string, error) {
	return versions(path)
}

// Query returns the directory in the local download cache
// holding the root of mod's source tree.
// It downloads the module if needed.
func ServerQuery(path string, version string) ([]module.Version, error) {
	info, err := Query(path, version, nil)
	if err != nil {
		return nil, err
	}
	return required(path, info.Version)
}

func required(path string, version string) ([]module.Version, error) {
	reqs := Reqs()
	mod := module.Version{Path: path, Version: version}
	list, err := reqs.Required(mod)
	if err != nil {
		return nil, err
	}

	//fmt.Printf("required %s/%s module list %v\n", path, version, list)
	return list, nil
}
