package proxy

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"cmd/go/internal/module"
	"cmd/go/internal/vgo"
)

const (
	goPathEnv   = "GOPATH"
	homeEnv     = "HOME"
	vgoCacheDir = "src/v/cache/"
)

const (
	listSuffix    = "/@v/list"
	zipSuffix     = ".zip"
	zipHashSuffix = ".ziphash"
	infoSuffix    = ".info"
	modSuffix     = ".mod"

	latestVersion = "latest"
)

var vgoRoot string

type proxyHandler struct {
	fileHandler http.Handler
}

func newProxyHandler(rootDir string) http.Handler {
	proxy := &proxyHandler{}
	proxy.fileHandler = http.FileServer(http.Dir(rootDir))
	return proxy
}

// ServeHTTP serve http
func (p *proxyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	url := r.URL.Path

	fmt.Printf("GET %s\n", url)
	if strings.HasSuffix(url, listSuffix) {
		listHandler(url, w, r)
		return
	}

	p.fetchStaticFile(url, w, r)
}

func (p *proxyHandler) fetchStaticFile(url string, w http.ResponseWriter, r *http.Request) {
	fullPath := filepath.Join(vgoRoot, url)
	if pathExist(fullPath) {
		p.fileHandler.ServeHTTP(w, r)
		return
	}

	fmt.Println("fetch file from remote host")

	var err error
	if strings.HasSuffix(url, infoSuffix) {
		err = p.fetch(url, infoSuffix)
	} else if strings.HasSuffix(url, zipSuffix) {
		err = p.fetch(url, zipSuffix)
	} else if strings.HasSuffix(url, zipHashSuffix) {
		err = p.fetch(url, zipHashSuffix)
	} else if strings.HasSuffix(url, modSuffix) {
		err = p.fetch(url, modSuffix)
	}

	if err != nil {
		w.WriteHeader(404)
		w.Write([]byte(err.Error()))
		return
	}

	p.fileHandler.ServeHTTP(w, r)
}

func pathExist(filePath string) bool {
	_, err := os.Stat(filePath)
	if err != nil && os.IsNotExist(err) {
		return false
	}
	return true
}

func (p *proxyHandler) fetch(filePath string, suffix string) error {
	url := filePath[:len(filePath)-len(suffix)]
	paths := strings.Split(url, "/@v/")

	mod := getPath(paths)
	ver := getVersion(paths)

	var err error
	switch suffix {
	case zipSuffix, zipHashSuffix:
		_, err = zipFetch(mod, ver)
	case infoSuffix, modSuffix:
		_, err = infoQuery(mod, ver)
	}

	return err
}

func zipFetch(mod string, ver string) (string, error) {
	dir, err := vgo.Fetch(mod, ver)
	if err != nil {
		fmt.Printf("\tdownload zip file failed: %v", err)
	} else {
		fmt.Printf("\tdownload zip file into dir %s\n", dir)
	}
	return dir, err
}

func infoQuery(mod string, ver string) ([]module.Version, error) {
	list, err := vgo.Query(mod, ver)
	if err != nil {
		fmt.Printf("\tquery module info failed: %v\n", err)
	} else {
		fmt.Printf("\tquery module info list: %v\n", list)
	}
	return list, err
}

func getPath(paths []string) string {
	return paths[0][1:]
}

func getVersion(paths []string) string {
	ver := latestVersion
	if len(paths) > 1 {
		ver = paths[1]
	}
	return ver
}

func listHandler(filePath string, w http.ResponseWriter, r *http.Request) {
	url := filePath
	mod := url[1 : len(url)-len(listSuffix)]
	list, err := infoQuery(mod, latestVersion)
	if err != nil {
		w.WriteHeader(200)
		w.Write([]byte(""))
		return
	}

	var versions []string

	for _, l := range list {
		versions = append(versions, l.Version)
	}

	w.WriteHeader(200)
	w.Write([]byte(strings.Join(versions, "\n")))
}

// Serve proxy serve
func Serve() {
	pathEnv := os.Getenv(goPathEnv)
	if pathEnv == "" {
		pathEnv = filepath.Join(os.Getenv(homeEnv), "go")
	}

	paths := strings.Split(pathEnv, string(os.PathListSeparator))
	gopath := paths[0]
	vgo.InitProxy(gopath)

	vgoRoot = filepath.Join(gopath, vgoCacheDir)
	h := newProxyHandler(vgoRoot)
	url := ":9090"
	fmt.Printf("start go mod proxy server at %s\n", url)
	err := http.ListenAndServe(url, h)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}
