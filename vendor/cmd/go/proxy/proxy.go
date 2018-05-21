package proxy

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"cmd/go/internal/vgo"
)

const (
	goPathEnv   = "GOPATH"
	homeEnv     = "HOME"
	vgoCacheDir = "src/v/cache/"
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
	fmt.Println(r.URL.Path)

	fullPath := filepath.Join(vgoRoot, r.URL.Path)
	if !pathExist(fullPath) {
		err := fetch(r.URL.Path)
		if err != nil {
			fmt.Println(err)
			w.WriteHeader(404)
			w.Write([]byte(err.Error()))
			return
		}
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

const (
	zipSuffix = ".zip"
)

func fetch(filePath string) error {
	strs := strings.Split(filePath, "/@v/")

	path := strs[0][1:]
	ver := "latest"
	if len(strs) > 1 {
		l := len(strs[1])
		ver = strs[1][:l - len(zipSuffix)]
	}


	dir, err := vgo.Fetch(path, ver)
	fmt.Printf("fetch module %s %s into dir %s\n", path, ver, dir)
	return err
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
