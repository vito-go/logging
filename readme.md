# logging
- ELK日志搜索终结者
- 无需数据库，支持超大（GB级别）**全链路日志**毫秒级别搜索。
- 更新log在线查看技术架构,由ajax短轮询升级为SSE推送.
- 支持跳转日志节点
- 支持反向代理日志节点

### 项目结构示意图
<img src="images/logging.png">

### API
- Init:   通用的初始化方式, http服务启动前进行调用。
- RegisterGin： 低层级的初始化方式，一般推荐使用Init。

### Usage
- main.go

```go

package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/vito-go/logging"
	"github.com/vito-go/logging/tid"
	"github.com/vito-go/mylog"
)

func main() {
	port := flag.Int("p", 9899, "specrify port ")
	flag.Parse()
	engine := gin.Default()
	appName:="chat"
	logInfoPath := "chat.log"
	logErrPath := "chat-err.log"
	fInfo, err := os.OpenFile(logInfoPath, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0644)
	if err != nil {
		panic(err)
	}
	fErr, err := os.OpenFile(logInfoPath, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0644)
	if err != nil {
		panic(err)
	}
	mylog.Init(fInfo, io.MultiWriter(fInfo, fErr), io.MultiWriter(fInfo, fErr), "tid")
	unilogServerAddr:=""// 单机版本，分布式注册中心地址可为空
	logging.Init(engine, *port, unilogServerAddr, logging.Config{
		APPName:     appName,
		Token:       "abc123",
		LogInfoPath: logInfoPath,
		LogErrPath:  logErrPath,
		TidPattern:  `"tid":(\d+)`,
	})
	engine.GET("/hello", func(ctx *gin.Context) {
		ctx.Set("tid", tid.Get())
		mylog.Ctx(ctx).WithField("path", ctx.Request.URL.Path).Info("request==>")
		ctx.JSON(200, "hello")
	})
	engine.Run(fmt.Sprintf(":%d", *port))
}

```

```shell
go run ./main.go
```
> http://127.0.0.1:9899/universe/api/v1/im/unilog/chat/chat.log
    -  input the login token: abc123

<img src="images/login.png">
- we can see the log stream
<img src="images/log.png">


#### 通过tid对日志全链路进行搜索
> http://127.0.0.1:9899/universe/api/v1/im/unilog/chat/tid-search

<img src="images/tid-search.png">
