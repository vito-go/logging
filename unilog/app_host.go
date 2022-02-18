package unilog

import (
	"context"
	"sort"
	"strings"
	"sync"

	"github.com/vito-go/mylog"
)

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
	return getHostByCode(appName, code)
}

// chooseOneHostByAppName 随机选择一个ip一般用于一个节点的服务查看滚动日志.
func chooseOneHostByAppName(appName string) string {
	appHostGlobal.mux.Lock()
	defer appHostGlobal.mux.Unlock()

	ipCodeMap := appHostGlobal.data[appName].ipCodeMap
	for host := range ipCodeMap {
		return host
	}
	return ""
}

// GetHosts 获取一个app的所有ip
func GetHosts(appName string) []string {
	appHostGlobal.mux.Lock()
	defer appHostGlobal.mux.Unlock()

	data, ok := appHostGlobal.data[appName]
	if !ok {
		return nil
	}
	var ips []string
	for ip := range data.ipCodeMap {
		ips = append(ips, ip)
	}
	return ips
}

type appHosts struct {
	App   string
	Hosts []string
}

// GetAllAppHosts 获取所有app的所有ip
func GetAllAppHosts() []appHosts {
	appHostGlobal.mux.Lock()
	defer appHostGlobal.mux.Unlock()
	var result = make([]appHosts, 10)
	for app, info := range appHostGlobal.data {
		hosts := make([]string, 0, len(info.ipCodeMap))
		for host := range info.ipCodeMap {
			hosts = append(hosts, host)
		}
		sort.Slice(hosts, func(i, j int) bool {
			return hosts[i] < hosts[j]
		})
		result = append(result, appHosts{
			App:   app,
			Hosts: hosts,
		})
	}
	sort.Slice(result, func(i, j int) bool {
		return strings.ToLower(result[i].App) < strings.ToLower(result[j].App)
	})
	return result
}

// GetAllAppHostMap 获取所有app的所有ip, map[app]hosts
func GetAllAppHostMap() map[string][]string {
	appHostGlobal.mux.Lock()
	defer appHostGlobal.mux.Unlock()
	var result = make(map[string][]string, 10)
	for app, info := range appHostGlobal.data {
		hosts := make([]string, 0, len(info.ipCodeMap))
		for host := range info.ipCodeMap {
			hosts = append(hosts, host)
		}
		result[app] = hosts
	}
	return result
}

// DelHost del a host when the node break.
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
			"appName", appName, "Host", host).Info("delete the cluster node")
		return
	}
	mylog.Ctx(ctx).WithField("Host", host).Warn("Can not Del Host. 未找到服务:", appName)
}

// Add 插入, 返回ipCode, true代表添加了
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

func getHostByCode(appName string, code int64) string {
	appHostGlobal.mux.Lock()
	defer appHostGlobal.mux.Unlock()

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
