package proxy

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

const (
	goPathEnv   = "GOPATH"
	homeEnv     = "HOME"
	vgoCacheDir = "src/v/cache/"
)

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
	p.fileHandler.ServeHTTP(w, r)
}

// Serve proxy serve
func Serve() {
	pathEnv := os.Getenv(goPathEnv)
	if pathEnv == "" {
		pathEnv = filepath.Join(os.Getenv(homeEnv), "go")
	}

	paths := strings.Split(pathEnv, string(os.PathListSeparator))
	vgoRoot := filepath.Join(paths[0], vgoCacheDir)
	h := newProxyHandler(vgoRoot)
	url := ":9090"
	fmt.Printf("start go mod proxy server at %s\n", url)
	err := http.ListenAndServe(url, h)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}
