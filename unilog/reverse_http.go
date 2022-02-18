package unilog

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync"
)

type nginx struct {
	mux sync.RWMutex
	rps map[string]*httputil.ReverseProxy // host
}

var nginxGlobal = nginx{
	mux: sync.RWMutex{},
	rps: map[string]*httputil.ReverseProxy{},
}

func (n *nginx) AddHostProxy(host string) (bool, error) {
	nginxGlobal.mux.Lock()
	defer nginxGlobal.mux.Unlock()
	if _, ok := n.rps[host]; ok {
		return false, nil
	}
	u, err := url.Parse(fmt.Sprintf("http://%s", host))
	if err != nil {
		return false, err
	}
	proxy := httputil.NewSingleHostReverseProxy(u)
	n.rps[host] = proxy
	return true, nil
}
func (n *nginx) GetProxy(host string) (*httputil.ReverseProxy, bool) {
	n.mux.RLock()
	defer n.mux.RUnlock()
	p, ok := n.rps[host]
	return p, ok
}

// reverse  浏览器客户端可能出现这个问题net::ERR_INCOMPLETE_CHUNKED_ENCODING 200 (OK)
// 可能是nginx的问题 需要修改nginx配置
func reverse(w http.ResponseWriter, r *http.Request, host string) bool {
	if host == "" {
		return false
	}
	if p, ok := nginxGlobal.GetProxy(host); ok {
		p.ServeHTTP(w, r)
		return true
	}
	return false
}

func redirect(w http.ResponseWriter, r *http.Request, host string, app string, logName string) bool {
	if host == "" || app == "" || logName == "" {
		return false
	}
	w.Header().Set("Cache-Control", "no-cache")                            // 必须设置无缓存，不然跳转到以前的ip。
	www := fmt.Sprintf("http://%s%s/%s/%s", host, _basePath, app, logName) // appName logName)
	http.Redirect(w, r, www, http.StatusMovedPermanently)
	return true
}
