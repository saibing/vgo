# vgoproxy

vgo包管理工具的代理服务器，支持常见的go语言库托管网站，如：

```bash
github.com
gopkg.in
golang.org
...
```
`

## 服务端安装部署

```bash
$ go get github.com/saibing/vgo
$ vgo
start vgo proxy server at http://127.0.0.1:9090
```

vgoproxy本身采用了[vgo](https://github.com/golang/vgo)的原型代码，在vgo代码的基础增加了proxy功能, 所以vgoproxy下载与管理包的原理与要求与vgo程序相同:


## 客户端配置

在客户端环境，配置很简单了，在使用vgo管理包的go语言工程环境中，设置环境变量：

```bash
export GOPROXY=http://127.0.0.1:9090
```
