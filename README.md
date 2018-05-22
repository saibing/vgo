# vgoproxy

vgo包管理工具的代理服务器，支持常见的go语言库托管网站，如：github.com, gopkg.in, golang.org,

另外也支持华为内部的几个git托管网站：rnd-isource.huawei.com, rnd-github.huawei.com, code.huawei.com。

## 安装部署

```bash
$ go get -u -v -insecure rnd-github.huawei.com/go/vgoproxy
$ vgoproxy
start vgo proxy server at http://127.0.0.1:9090
```

## 使用环境配置

在使用vgo管理包的go语言工程环境中，设置环境变量：

```bash
export GOPROXY=http://127.0.0.1:9090
```

然后使用vgo命令即可以连接vgo proxy下载第三方的开源库。

## 注意

个人开发，精力有限，仅限于华为内部项目小组使用。