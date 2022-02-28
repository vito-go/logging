// Package unilog .分布式日志注册中心.
// 全局对外可用两个接口:
// GoStart: 分布式日志注册中心
// GetHostByCode: 根据服务(集群)名称和code码 获取对应ip.
//
// 节点ip对应code码生成规则: 优先默认取其节点ip最后一组数字,比如 comment集群  节点ip: 192.168.1.105 那么code码对应为105.
// 假如comment集群已经存在i节点ip 192.168.2.105(先注册code码为105),那么节点ip: 192.168.1.105则将重新计算,从1开始寻找ip码空位.
package unilog

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"net/rpc"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/vito-go/mylog"

	"github.com/vito-go/logging"
	"github.com/vito-go/logging/tid"
	"github.com/vito-go/logging/unilogrpc"
)

type Server struct{ ctx context.Context }

// Register 分布式注册rpc方法
func (s *Server) Register(req *unilogrpc.UnilogRegisterReq) (*int64, error) {
	ctx := s.ctx
	code, isAdd := appHostGlobal.add(req.APPName, req.Host, req.CodeInt) // 返回true对方
	if isAdd {
		mylog.Ctx(ctx).WithField("req", req).Info("cluster node ==>>")
	}
	added, err := nginxGlobal.AddHostProxy(req.Host)
	if err != nil {
		mylog.Ctx(ctx).WithField("req", req).Errorf("nginxGlobal.AddHostProxy error. err: %s", err.Error())
		return &code, nil
	}
	if added {
		mylog.Ctx(ctx).WithField("req", req).Info("nginxGlobal.AddHostProxy successfully")
	}
	return &code, nil
}

// _basePath 默认根路径，应当和logging的节点根路径保持一致. Change is with the path in GoStart.
var _basePath = "/logging"

// GoStart start the unilog. logFunc 根据app获取info日志和err日志文件名，不应包含路径. 用来做日志导航。
func GoStart(engine *gin.Engine, rpcServerAddr string, path logging.BasePath, logFunc LogInfoNameFunc, appNames ...string) {
	appNameList = appNames
	if path != "" {
		logging.MustCheckBasePath(path)
		_basePath = string(path)
	}
	start(engine, rpcServerAddr, logFunc)
}
func start(engine *gin.Engine, rpcServerAddr string, logFunc LogInfoNameFunc) {
	ctx := context.WithValue(context.Background(), "tid", tid.Get())
	listener, err := net.Listen("tcp", rpcServerAddr)
	if err != nil {
		// todo
		mylog.Ctx(ctx).WithField("unilog-addr", rpcServerAddr).Error(err.Error())
		return
	}
	rpcSrv := rpc.NewServer()
	err = unilogrpc.RegisterUnilogServer(rpcSrv, &Server{ctx: ctx})
	if err != nil {
		mylog.Ctx(ctx).WithField("unilog-addr", rpcServerAddr).Errorf("unilog server register error:", err.Error())
		return
	}
	mylog.Ctx(ctx).WithField("unilog-addr", rpcServerAddr).Info("unilog distributed systems cluster start.")
	engine.Any(filepath.ToSlash(filepath.Join(_basePath, ":app", "*log")), tidUniAPPLog) // 反向代理
	engine.GET(_basePath, tidUnilogGet)                                                  // tid search界面
	engine.POST(_basePath, tidUnilogPost)                                                // post 查询tid

	navi := &logNavi{getLogNameByApp: DefaultLogInfoNameFunc}
	if logFunc != nil {
		navi.getLogNameByApp = logFunc
	}
	engine.GET(filepath.ToSlash(filepath.Join(_basePath, "log-navi")), navi.LoggingNavi) // log 导航                                    // post 查询tid
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				mylog.Ctx(ctx).WithField("unilog-addr", rpcServerAddr).Errorf("unilog distributed systems cluster is over. err:", err.Error())
				return
			}

			go func(conn net.Conn) {
				defer conn.Close()
				mylog.Ctx(ctx).Info("unilog rpc: receive a new client", conn.RemoteAddr().String())
				httpPort, err := checkAndGetPort(conn)
				if err != nil {
					mylog.Ctx(ctx).WithField("remote_addr", conn.RemoteAddr()).Warn("unilog rpc: get http port error:", err.Error())
					return
				}
				rpcSrv.ServeConn(conn)
				mylog.Ctx(ctx).Info("unilog rpc: client is over:", conn.RemoteAddr().String())
				var ip string
				if ss := strings.Split(conn.RemoteAddr().String(), ":"); len(ss) > 0 {
					ip = ss[0]
				}
				appHostGlobal.DelHost(ctx, fmt.Sprintf("%s:%d", ip, httpPort))
			}(conn)
		}
	}()
}

func checkAndGetPort(conn net.Conn) (uint32, error) {
	// protocol 协议   端口号协议： lsh:<portCode>
	const protocol = "lsh:"

	buf := make([]byte, 8) // 获取端口号
	_, err := io.ReadFull(conn, buf)
	if err != nil {
		return 0, err
	}
	if s := string(buf[:4]); s != protocol {
		return 0, fmt.Errorf("logging rpc: client protocol error. it's should be `%s` ,but it's `%s`", protocol, s)
	}
	httpPort := binary.BigEndian.Uint32(buf[4:8])
	return httpPort, nil
}
