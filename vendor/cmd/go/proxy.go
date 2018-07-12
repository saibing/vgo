package Main

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"bytes"
	"encoding/json"
	"io/ioutil"
	"os/exec"
	"sort"
	"sync"
	"cmd/go/internal/modfetch"
	"cmd/go/internal/module"
	"cmd/go/internal/modload"
)

const (
	goPathEnv   = "GOPATH"
	homeEnv     = "HOME"
	webRoot     = "src/mod/cache/download/"
	vgoModDir   = "src/mod"
)

const (
	listSuffix    = "/@v/list"
	latestSuffix  = "/@latest"
	zipSuffix     = ".zip"
	zipHashSuffix = ".ziphash"
	infoSuffix    = ".info"
	modSuffix     = ".mod"

	latestVersion = "latest"
)

type Config struct {
	GoPath    string            `json:"gopath"`
	HTTPSites []string          `json:"http"`
	Replace   map[string]string `json:"replace"`
	SortKeys  []string          `json:"sortKeys"`
}

func (cfg *Config) Init() {
	for k := range cfg.Replace {
		cfg.SortKeys = append(cfg.SortKeys, k)
	}

	sort.Slice(cfg.SortKeys, func(i, j int) bool {
		return len(cfg.SortKeys[i]) >= len(cfg.SortKeys[j])
	})

	modfetch.HTTPSites = cfg.HTTPSites
}

func (cfg *Config) String() string {
	data, _ := json.MarshalIndent(cfg, "", "   ")
	return string(data)
}

var fullWebRoot string
var vgoModRoot string

type proxyHandler struct {
	cfg         *Config
	fileHandler http.Handler
}

func newProxyHandler(rootDir string, cfg *Config) http.Handler {
	proxy := &proxyHandler{cfg: cfg, fileHandler: http.FileServer(http.Dir(rootDir))}
	return proxy
}

var allMutex sync.Mutex
var modMutex sync.Mutex
var zipMutex sync.Mutex

// ServeHTTP serve http
func (p *proxyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	//allMutex.Lock()
	//defer allMutex.Unlock()

	originURL := r.URL.Path
	replaced := p.replace(r)

	url := r.URL.Path

	logRequest(fmt.Sprintf("GET %s from %s", url, r.RemoteAddr))
	if replaced {
		logRequest(fmt.Sprintf("Origin url %s", originURL))
	}

	if strings.HasSuffix(url, listSuffix) {
		listHandler(url, w)
		return
	}

	if strings.HasSuffix(url, latestSuffix) {
		p.latestVersionHandler(url, w, r)
		return
	}

	p.fetchStaticFile(originURL, w, r)
}

func (p *proxyHandler) replace(r *http.Request) bool {
	k, v := p.findReplace(r.URL.Path)
	if k == "" || v == "" {
		return false
	}

	k = "/" + k
	v = "/" + v

	r.URL.Path = v + r.URL.Path[len(k):]
	return true
}

func (p *proxyHandler) findReplace(url string) (string, string) {
	for _, k := range p.cfg.SortKeys {
		if strings.HasPrefix(url, "/"+k) {
			return k, p.cfg.Replace[k]
		}
	}

	return "", ""
}

func (p *proxyHandler) latestVersionHandler(url string, w http.ResponseWriter, r *http.Request) {
	paths := strings.Split(url, "/@")
	mod := getPath(paths)
	ver := getVersion(paths)

	revInfo, err := modload.ServerModule(mod, ver)
	if err != nil {
		logError("go: %v", err)
		w.WriteHeader(404)
		w.Write([]byte(err.Error()))
		return
	}

	logInfo("go: the latest version: %v", *revInfo)

	data, err := json.Marshal(revInfo)
	if err != nil {
		logError("go: %v", err)
		w.WriteHeader(404)
		w.Write([]byte(err.Error()))
		return
	}

	w.WriteHeader(200)
	w.Write(data)
}

func (p *proxyHandler) fetchStaticFile(originURL string, w http.ResponseWriter, r *http.Request) {
	url := r.URL.Path
	fullPath := filepath.Join(fullWebRoot, url)
	if pathExist(fullPath) {
		p.downloadFile(originURL, w, r)
		return
	}

	logInfo("go: fetch file from remote host: %s", url)

	var err error
	if strings.HasSuffix(url, infoSuffix) {
		err = p.fetch(url, infoSuffix)
	} else if strings.HasSuffix(url, zipSuffix) {
		err = p.fetch(url, zipSuffix)
	} else if strings.HasSuffix(url, zipHashSuffix) {
		err = p.fetch(url, zipHashSuffix)
	} else if strings.HasSuffix(url, modSuffix) {
		err = p.fetch(url, modSuffix)
	} else {
		p.fileHandler.ServeHTTP(w, r)
		return
	}

	if err != nil {
		write404Error("go: fetch file failed %s", w, err)
		return
	}

	p.downloadFile(originURL, w, r)
}

func write404Error(format string, w http.ResponseWriter, err error) {
	logError(format, err.Error())
	w.WriteHeader(404)
	w.Write([]byte(err.Error()))
}

func (p *proxyHandler) downloadFile(originURL string, w http.ResponseWriter, r *http.Request) {
	url := r.URL.Path

	if originURL == url {
		p.fileHandler.ServeHTTP(w, r)
		return
	}

	if strings.HasSuffix(url, modSuffix) {
		p.downloadMod(originURL, w, r)
		return
	}

	if strings.HasSuffix(url, zipSuffix) {
		p.downloadZip(originURL, w, r)
		return
	}

	p.fileHandler.ServeHTTP(w, r)
}

func (p *proxyHandler) downloadZip(originURL string, w http.ResponseWriter, r *http.Request) {
	zipMutex.Lock()
	defer zipMutex.Unlock()

	originPath := filepath.Join(fullWebRoot, originURL)
	r.URL.Path = originURL
	logInfo("go: download zip file: %s", originPath)
	if pathExist(originPath) {
		logInfo("go: zip file %s already exist", originPath)
		p.fileHandler.ServeHTTP(w, r)
		return
	}

	logInfo("go: zip file %s does not exist", originPath)
	targetDir := filepath.Dir(originPath)
	if !pathExist(targetDir) {
		logInfo("go: mkdir %s", targetDir)
		err := os.MkdirAll(targetDir, fileMode)
		if err != nil {
			write404Error("go: read mod file parent targetDir failed %s", w, err)
			return
		}
	}

	targetFileName := filepath.Base(originPath)
	key, value := p.findReplace(originURL)

	targetNoExt := targetFileName[:len(targetFileName)-len(zipSuffix)]
	sourceDir := filepath.Join(vgoModRoot, value+"@"+targetNoExt)

	keys := strings.Split(key, string(os.PathSeparator))
	if len(keys) <= 1 {
		err := fmt.Errorf("invalid module path %s", key)
		write404Error("go: copy file failed: %s", w, err)
		return
	}

	copyTargetDir := filepath.Join(targetDir, key[:len(key)-len(keys[len(keys)-1])])
	err := copyDir(sourceDir, copyTargetDir)
	if err != nil {
		removeDir(copyTargetDir)
		write404Error("go: copy file failed: %s", w, err)
		return
	}

	zipSourceDir := key + "@" + targetNoExt
	err = zipDir(targetDir, zipSourceDir, targetFileName)
	if err != nil {
		removeFile(filepath.Join(targetDir, targetFileName))
		write404Error("go: zip file failed: %s", w, err)
		return
	}

	removeDir(filepath.Join(targetDir, keys[0]))

	p.fileHandler.ServeHTTP(w, r)
}

const (
	fileMode = 0755
)

func removeDir(dir string) error {
	logInfo("go: remove dir %s", dir)
	return os.RemoveAll(dir)
}

func removeFile(filePath string) error {
	logInfo("go: remove file %s", filePath)
	return os.Remove(filePath)
}

func copyDir(source string, target string) error {
	if pathExist(target) {
		return nil
	}

	logInfo("go: mkdir %s", target)
	err := os.MkdirAll(target, fileMode)
	if err != nil {
		return err
	}

	shell := fmt.Sprintf("cp -r %s %s", source, target)
	return execShell(shell)
}

func execShell(s string) error {
	logInfo("go: %s", s)

	cmd := exec.Command("/bin/bash", "-c", s)
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		return err
	}
	logInfo("go: %s", out.String())
	return nil
}

func zipDir(workDir string, zipSourceDir string, target string) error {
	shell := fmt.Sprintf("cd %s; zip -r %s %s", workDir, target, zipSourceDir)
	return execShell(shell)
}

func (p *proxyHandler) downloadMod(originURL string, w http.ResponseWriter, r *http.Request) {
	modMutex.Lock()
	defer modMutex.Unlock()

	fullPath := filepath.Join(fullWebRoot, r.URL.Path)
	originPath := filepath.Join(fullWebRoot, originURL)
	r.URL.Path = originURL
	logInfo("go: download mod file: %s", originPath)
	if pathExist(originPath) {
		logInfo("go: mod file %s already exist", originPath)
		p.fileHandler.ServeHTTP(w, r)
		return
	}

	dir := filepath.Dir(originPath)
	if !pathExist(dir) {
		err := os.MkdirAll(dir, fileMode)
		if err != nil {
			write404Error("go: read mod file parent dir failed %s", w, err)
			return
		}
	}

	logInfo("go: create mod file: %s", originPath)
	src, err := ioutil.ReadFile(fullPath)
	if err != nil {
		write404Error("go: read mod file failed %s", w, err)
		return
	}

	k, v := p.findReplace(originURL)
	newContent := bytes.Replace(src, []byte("module "+v), []byte("module "+k), -1)
	err = ioutil.WriteFile(originPath, []byte(newContent), fileMode)
	if err != nil {
		write404Error("go: create mod file failed: %s", w, err)
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
	dir, _, err := modload.ServerFetch(mod, ver)
	if err != nil {
		logError("go: download zip file failed: %v", err)
	} else {
		logInfo("go: download zip file into dir %s", dir)
	}
	return dir, err
}

func listVersions(mod string) ([]string, error) {
	versions, err := modload.ServerVersions(mod)
	if err != nil {
		logError("go: list version failed: %v", err)
	} else {
		logInfo("go: version list: %v", versions)
	}

	return versions, err
}

func infoQuery(mod string, ver string) ([]module.Version, error) {
	list, err := modload.ServerQuery(mod, ver)
	if err != nil {
		logError("go: query %s/%s module info failed: %v", mod, ver, err)
	} else {
		logInfo("go: %s/%s module info list: %v", mod, ver, list)
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

func listHandler(filePath string, w http.ResponseWriter) {
	url := filePath
	mod := url[1 : len(url)-len(listSuffix)]
	versions, err := listVersions(mod)
	if err != nil {
		w.WriteHeader(404)
		w.Write([]byte(""))
		return
	}

	w.WriteHeader(200)
	w.Write([]byte(strings.Join(versions, "\n")))
}

// Serve proxy serve
func Serve(ip string, port string, cfg *Config) {
	if cfg.GoPath != "" {
		os.Setenv(goPathEnv, cfg.GoPath)
	}

	pathEnv := os.Getenv(goPathEnv)
	if pathEnv == "" {
		pathEnv = filepath.Join(os.Getenv(homeEnv), "go")
	}

	paths := strings.Split(pathEnv, string(os.PathListSeparator))
	gopath := paths[0]
	modload.InitProxy(gopath)

	fullWebRoot = filepath.Join(gopath, webRoot)
	vgoModRoot = filepath.Join(gopath, vgoModDir)
	h := newProxyHandler(fullWebRoot, cfg)
	url := ip + ":" + port
	logInfo("go config: \n%s", cfg)
	logInfo("start go proxy server at %s", url)
	err := http.ListenAndServe(url, h)
	if err != nil {
		logError("listen serve failed, %v", err)
	}
}
