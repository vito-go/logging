// Package tid 生成traceId进行日志追踪，基于时间纳秒级别。本机全局唯一.
package tid

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"log"
	"net"
	"net/rpc"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/vito-go/mylog"

	"github.com/vito-go/logging/unilogrpc"
)

type tide struct {
	mux sync.Mutex
	t   int64
}

func newTid() *tide {
	return &tide{mux: sync.Mutex{}}
}

var tid = newTid()

// rpcRetryLimit 最多尝试100次（约为10分钟）
const rpcRetryLimit = 100

// _ipCode 全局本机ip所对应的code.
var _ipCode = ipCodeInit()

func Get() int64 {
	return tid.get()
}

// Register can only do once.
// unilogAddr 分布式日志统一中心地址. httpPort 用来向注册中心报告本机http服务port.
func Register(appName string, httpPort int, unilogAddr string) error {
	if unilogAddr == "" {
		return errors.New("unilogAddr empty")
	}
	if _ipCode == 0 {
		return errors.New("_ipCode is zero")
	}
	go run(appName, httpPort, unilogAddr)
	return nil
}

func ipCodeInit() int64 {
	priIP, err := getPrivateIP()
	if err != nil {
		log.Println(err)
		return 0
	}
	var ipCodeStr string
	if ss := strings.Split(priIP, "."); len(ss) == 4 {
		ipCodeStr = ss[3]
	}
	ipCode, err := strconv.ParseInt(ipCodeStr, 10, 64)
	if err != nil {
		log.Printf("ipcode init error. priIP=%s ipCodeStr=%s err=%s\n", priIP, ipCodeStr, err.Error())
	}
	return ipCode
}

// start. 第一个for循环.
func run(appName string, httpPort int, unilogAddr string) {
	var conn net.Conn
	var err error
	var rpcCli *rpc.Client
	var retryCount int
	for {
		if retryCount >= rpcRetryLimit {
			mylog.Ctx(context.TODO()).Warnf("unilog重试次数超过%d, 节点终止", rpcRetryLimit)
			// 重试超过100次就终止
			return
		}
		conn, err = net.Dial("tcp", unilogAddr)
		if err != nil {
			// mylog.Ctx(context.TODO()).Errorf("unilog服务链接错误, 10s后重试验. err: %s", err.Error())
			time.Sleep(time.Second * 6)
			retryCount++
			continue
		}
		var portB = make([]byte, 8)
		copy(portB, "lsh:")
		binary.BigEndian.PutUint32(portB[4:], uint32(httpPort))
		_, err = conn.Write(portB) // 将服务的端口号发送给注册中心，以便客户端停止的时候 服务端能 删除该服务
		if err != nil {
			time.Sleep(time.Second * 3)
			retryCount++
			continue
		}
		// todo 服务端的响应
		rpcCli = rpc.NewClient(conn)
		unilogRPC := unilogrpc.NewUnilogCli(rpcCli)
		for {
			code := atomic.LoadInt64(&_ipCode)
			var ip string
			if s := conn.LocalAddr().String(); strings.Index(s, ":") != -1 {
				ip = s[:strings.Index(s, ":")]
			} else {
				mylog.Ctx(context.Background()).WithField("LocalAddr", s).Error("there is no colon :")
			}
			respCode, err := unilogRPC.Register(&unilogrpc.UnilogRegisterReq{
				APPName: appName,
				Host:    fmt.Sprintf("%s:%d", ip, httpPort),
				CodeInt: code,
			})
			if err != nil {
				// 大概率是unilog服务器中断
				break
			}
			atomic.StoreInt64(&_ipCode, *respCode)
			retryCount = 0 // 正常的重启
			time.Sleep(time.Second * 3)
		}
	}
}

func (u *tide) get() int64 {
	u.mux.Lock()
	defer u.mux.Unlock()
	// go1.14  time.Now().UnixMicro undefined (type time.Time has no field or method UnixMicro)
	// t := time.Now().UnixMicro()
	t := time.Now().UnixNano()/1e3*1e3 + atomic.LoadInt64(&_ipCode)
	if u.t == t {
		t++
	}
	u.t = t
	return t
}
