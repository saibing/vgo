package vgo


// Fetch returns the directory in the local download cache
// holding the root of mod's source tree.
// It downloads the module if needed.
func Fetch(path string, version string) (dir string, err error) {
	mod := module.Version{Path: path, Version: version}
	return fetch(mod)
}