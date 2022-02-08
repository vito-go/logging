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
	"sync"

	"github.com/gin-gonic/gin"

	"github.com/vito-go/mylog"

	"github.com/vito-go/logging"
	"github.com/vito-go/logging/tid"
	"github.com/vito-go/logging/unilogrpc"
)

type Server struct{}

// Register 分布式注册rpc方法
func (s *Server) Register(req *unilogrpc.UnilogRegisterReq) (*int64, error) {
	code, isAdd := appHostGlobal.add(req.APPName, req.Host, req.CodeInt) // 返回true对方
	if isAdd {
		mylog.Ctx(context.Background()).WithField("req", req).Info("集群节点 ==>>")
	}
	added, err := nginxGlobal.AddHostProxy(req.Host)
	if err != nil {
		mylog.Ctx(context.Background()).WithField("req", req).Errorf("nginxGlobal.AddHostProxy error. err=%s", err.Error())
		return &code, nil
	}
	if added {
		mylog.Ctx(context.Background()).WithField("req", req).Info("nginxGlobal.AddHostProxy successfully")
	}
	return &code, nil
}

// GoStart 启动unilog注册中心
func GoStart(engine *gin.Engine, rpcServerAddr string, appNames ...string) {
	appNameList = appNames
	start(engine, rpcServerAddr)
}
func start(engine *gin.Engine, rpcServerAddr string) {
	ctx := context.WithValue(context.Background(), "tid", tid.Get())
	listener, err := net.Listen("tcp", rpcServerAddr)
	if err != nil {
		// todo
		mylog.Ctx(ctx).WithField("unilog-addr", rpcServerAddr).Error(err.Error())
		return
	}
	rpcSrv := rpc.NewServer()
	err = unilogrpc.RegisterUnilogServer(rpcSrv, &Server{})
	if err != nil {
		mylog.Ctx(ctx).WithField("unilog-addr", rpcServerAddr).Errorf("server服务注册失败 err:", err.Error())
		return
	}
	mylog.Ctx(ctx).WithField("unilog-addr", rpcServerAddr).Info("unilog分布式日志搜索引擎已启动")
	engine.Any(filepath.ToSlash(filepath.Join(logging.BasePath, ":app", "*log")), tidUniAPPLog) // 反向代理
	engine.GET(logging.BasePath, tidUnilogGet)                                                  // app={app}&log={log} 跳转
	engine.POST(logging.BasePath, tidUnilogPost)                                                // post 查询tid
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				mylog.Ctx(ctx).WithField("unilog-addr", rpcServerAddr).Errorf("unilog分布式日志搜索引擎结束! err:", err.Error())
				return
			}
			// 端口号协议： lsh:<>
			buf := make([]byte, 8) // 获取端口号
			_, err = io.ReadFull(conn, buf)
			if err != nil {
				conn.Close()
				mylog.Ctx(ctx).WithField("remoteAddr", conn.RemoteAddr().String()).Error(err.Error())
				continue
			}
			// 不符合协议
			if string(buf[:4]) != "lsh:" {
				conn.Close()
				continue
			}
			mylog.Ctx(ctx).WithField("remoteAddr", conn.RemoteAddr().String()).Info("收到新的集群服务链接")
			httpPort := binary.BigEndian.Uint32(buf[4:8])
			go func(conn net.Conn) {
				defer conn.Close()
				rpcSrv.ServeConn(conn)
				mylog.Ctx(ctx).WithField("remoteAddr", conn.RemoteAddr().String()).Info("集群服务链接 结束！")
				var ip string
				if ss := strings.Split(conn.RemoteAddr().String(), ":"); len(ss) > 0 {
					ip = ss[0]
				}
				appHostGlobal.DelHost(ctx, fmt.Sprintf("%s:%d", ip, httpPort))
			}(conn)
		}
	}()
}

// appHost .
type appHost struct {
	mux      sync.RWMutex           // protect the data field.
	data     map[string]appHostCode // map[appName]appHostCode 后续考虑用链表标识? 不过ip量没有达到使用链表结构级别
	ipAppMap map[string]string      // map[host]appName
}

var appHostGlobal = appHost{mux: sync.RWMutex{}, data: map[string]appHostCode{}, ipAppMap: map[string]string{}}

type appHostCode struct {
	ipCodeMap      map[string]int64 // map[host]int64
	codeHostMap    map[int64]string // map[int64]host
	ipCodeExistMap map[int64]bool   // 编号是否存在
}

func GetHostByCode(appName string, code int64) string {
	return appHostGlobal.getHostByCode(appName, code)
}

// ChooseOneHostByAppName 随机选择一个ip一般用于一个节点的服务查看滚动日志.
func (a *appHost) ChooseOneHostByAppName(appName string) string {
	a.mux.Lock()
	defer a.mux.Unlock()
	if a == nil || a.data == nil {
		// 这种情况不会发生
		return ""
	}
	ie := appHostGlobal.data[appName].ipCodeExistMap
	for code := range ie {
		return appHostGlobal.data[appName].codeHostMap[code]
	}
	return ""
}

// DelHost 删除app ip节点.
func (a *appHost) DelHost(ctx context.Context, host string) {
	a.mux.Lock()
	defer a.mux.Unlock()
	if a == nil || a.data == nil {
		mylog.Ctx(ctx).Warn("nil appIp or nil a.data")
		// 这种情况不会发生
		return
	}
	appName, ok := a.ipAppMap[host]
	if !ok {
		mylog.Ctx(ctx).WithFields("appName", appName, "Host", host).Warnf(
			"Host not found.  a.data: %+v a.ipAppMap: %+v", a.data, a.ipAppMap)
		return
	}
	if d, ok := a.data[appName]; ok {
		code := d.ipCodeMap[host]
		delete(d.ipCodeMap, host)
		delete(d.ipCodeExistMap, code)
		delete(d.codeHostMap, code)
		delete(a.ipAppMap, host)
		mylog.Ctx(ctx).WithFields(
			"appName", appName, "Host", host).Info("删除集群节点")
		return
	}
	mylog.Ctx(ctx).WithField("Host", host).Warn("Can not Del Host. 未找到服务:", appName)
}

// Add true 插入, 返回ipCode. true代表添加了
func (a *appHost) add(appName, host string, codeInt int64) (int64, bool) {
	a.mux.Lock()
	defer a.mux.Unlock()
	if a == nil || a.data == nil {
		// 这种情况不会发生
		return 0, false
	}
	// 已存在的集群
	if d, ok := a.data[appName]; ok {
		var code int64
		code, ok = d.ipCodeMap[host]
		if ok {
			// 代表着如果曾经存储过ip, 那么就视为是最终定音的code
			return code, false
		}
		// 集群存在,code也存在.但是 和目前的ip不匹配 那么将该ip+1
		if codeInt == 0 || (a.data[appName].ipCodeExistMap[codeInt] && a.data[appName].codeHostMap[codeInt] != host) {
			code = getHostCode(a.data[appName].ipCodeExistMap)
		} else {
			code = codeInt
		}
		// 注意这里的逻辑
		a.data[appName].ipCodeExistMap[code] = true
		a.data[appName].ipCodeMap[host] = code
		a.data[appName].codeHostMap[code] = host
		a.ipAppMap[host] = appName
		return code, true
	}
	// 不存在的集群
	var code int64 = 1
	if codeInt != 0 {
		code = codeInt
	}
	a.data[appName] = appHostCode{
		ipCodeMap:      map[string]int64{host: code},
		codeHostMap:    map[int64]string{code: host},
		ipCodeExistMap: map[int64]bool{code: true},
	}
	a.ipAppMap[host] = appName
	return code, true
}

// Add true 插入, 返回ipCode.
func (a *appHost) getHostByCode(appName string, code int64) string {
	a.mux.Lock()
	defer a.mux.Unlock()
	if a == nil || a.data == nil {
		// 这种情况不会发生
		return ""
	}
	// 已存在的集群
	return appHostGlobal.data[appName].codeHostMap[code]
}

// getHostCode 获取一个ip对应的code.
// 集群存在,code也存在.但是 和目前的ip不匹配 那么将该ip+1
func getHostCode(m map[int64]bool) int64 {
	var i int64 = 1
	for {
		if !m[i] {
			return i
		}
		i++
	}
}
