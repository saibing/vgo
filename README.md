# vgoproxy

vgo包管理工具的代理服务器，支持常见的go语言库托管网站，如：github.com, gopkg.in, golang.org,

另外也支持华为内部的几个git托管网站：rnd-isource.huawei.com, rnd-github.huawei.com, code.huawei.com。

## 安装部署

```bash
$ go get -u -v -insecure rnd-github.huawei.com/go/vgoproxy
$ vgoproxy
start vgo proxy server at http://127.0.0.1:9090
```

## 注意
vgoproxy本身采用了[vgo](https://github.com/golang/vgo)的原型代码，在vgo代码的基础增加了proxy功能

所以vgoproxy下载与管理包的原理与要求与vgo程序相同:

### 配置git
vgoproxy使用git下载代码，需要外网上网的proxy权限, 请在$HOME/.gitconfig文件中增加:

```bash
[http]
    proxy=http://user:password@proxyhk.huawei.com:8080
    sslverify = false

[https]
    proxy=http://user:password@proxyhk.huawei.com:8080
    sslVerify=false

[http "http://code.huawei.com"]
    proxy=
    sslVerify=false

[http "http://rnd-github.huawei.com"]
    proxy=
    sslVerify=false

[http "http://rnd-isourceb.huawei.com"]
    proxy=
    sslVerify=false

[user]
    name = xxx
    email = xxx@huawei.com
```

### 配置.netrc文件

vgoproxy下载github.com的开源代码时，需要有api.github.com的帐号与权限，否则有下载过多的github.com项目时，会报limit的限制,

请在$HOME/.netrc增加如下配置：

```bash
machine api.github.com login saibing password 0ef4a5827997f8dxxxxxxxxxx6c97aeb7e
```

### 设置GOPATH环境变量

如果没有设置GOPATH, 会使用默认的GOPATH值：$HOME/go

vgoproxy会把第三方开源软件缓存在$GOPATH/src/v/cache目录下面。

但你不需要安装go编译器, vgoproxy不会编译任何go语言工程。事实上：

我已经把vgo的所有的命令行功能都屏蔽了，新提供了http server的功能。


## 使用环境配置

在客户端环境，配置很简单了，在使用vgo管理包的go语言工程环境中，设置环境变量：

```bash
export GOPROXY=http://127.0.0.1:9090
```

然后使用vgo命令即可以连接到vgo proxy下载第三方的开源库。因为vgoproxy的存在，

每个vgo客户端就不需要像vgoproxy安装部署一样，设置一堆的东西。

## 注意

个人开发，精力有限，仅限于华为内部项目小组使用。