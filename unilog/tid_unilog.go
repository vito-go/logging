package unilog

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/vito-go/mylog"
)

//go:embed unilog_tid.html
// appNameList
var unilogTidHtml string

//  appNameList 未来考虑走配置
var appNameList []string

type LogInfoNameFunc func(app string) (logInfo, logErr string)

// DefaultLogInfoNameFunc 默认的日志文件名称规则.
var DefaultLogInfoNameFunc = func(app string) (logInfo, logErr string) {
	return app + ".log", app + "-err.log"
}

// tidUnilogGet 分布式日志搜索路由入口. 打开tid搜索界面或者进行跳转. 与logging滚动查看日志(即 logClient )进行了解藕
func tidUnilogGet(w http.ResponseWriter, r *http.Request) {
	// 优先判断是否有app 和 log 参数，可以进行跳转
	b, _ := json.Marshal(appNameList)
	// 替换符加个单引号防止被格式化
	w.Header().Set("Cache-Control", "no-cache") // 必须设置无缓存，不然跳转到以前的ip。
	replacer := strings.NewReplacer("'{{appNameList}}'", string(b), "'{{BasePath}}'", _basePath)
	replacer.WriteString(w, unilogTidHtml)
	return
}

func tidUnilog(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		tidUnilogGet(w, r)
		return
	} else if r.Method == http.MethodPost {
		tidUnilogPost(w, r)
		return
	}
}

// tidUnilogPost post 请求获取日志.
func tidUnilogPost(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}
	tidStr := r.FormValue("tid")
	appName := r.FormValue("appName")
	ctx := r.Context()
	tidInt, err := strconv.ParseInt(tidStr, 10, 64)
	if err != nil || tidInt <= 0 {
		mylog.Ctx(ctx).WithField("addr", r.RemoteAddr).WithField("tid", tidStr).Warnf(
			"查看tid日志失败！tid错误")
		w.WriteHeader(http.StatusForbidden)
		return
	}
	host, err := getIpByTidStr(appName, tidStr)
	if err != nil {
		w.Write([]byte(err.Error()))
		return
	}
	if host == "" {
		w.Write([]byte("<h1>未找到对应集群节点</h1>"))
		return
	}
	tidURL := fmt.Sprintf("http://%s%s/%s/tid-search", host, _basePath, appName)
	postForm := url.Values{}
	postForm.Set("tid", tidStr)
	mylog.Ctx(ctx).WithFields("tidURL", tidURL, "postForm", postForm).Info(r.RemoteAddr, "搜索日志")
	response, err := http.PostForm(tidURL, postForm)
	if err != nil {
		mylog.Ctx(ctx).WithFields("tidURL", tidURL, "postForm", postForm).Error(err)
		w.Write([]byte(err.Error()))
		return
	}
	defer response.Body.Close()
	io.Copy(w, response.Body)

}

// tidUniAPPLog 分布式日志搜索路由入口. 与logging滚动查看日志(即 logClient )进行了解藕
func tidUniAPPLog(w http.ResponseWriter, r *http.Request) {
	// /_basePath/{app}/{log}
	appLogs := strings.Split(strings.TrimPrefix(r.URL.Path, _basePath+"/"), "/")
	if len(appLogs) != 2 {
		http.NotFound(w, r)
		return
	}
	app := appLogs[0]
	if host := chooseOneHostByAppName(app); host != "" {
		// 可以选择redirect 跳转 或者reverse反向代理，未来可以考虑走配置
		if reverse(w, r, host) {
			return
		}
	}
	b, _ := json.Marshal(appNameList)
	// 替换符加个单引号防止被格式化
	w.Header().Set("Cache-Control", "no-cache") // 必须设置无缓存，不然跳转到以前的ip。
	replacer := strings.NewReplacer("'{{appNameList}}'", string(b), "'{{BasePath}}'", _basePath)
	replacer.WriteString(w, unilogTidHtml)
	return

}

func getIpByTidStr(appName string, tidStr string) (string, error) {
	if len(tidStr) < 3 {
		return "", errors.New("tid有误 tid: " + tidStr)
	}
	codeStr := tidStr[len(tidStr)-3:]
	codeInt, err := strconv.ParseInt(codeStr, 10, 64)
	if err != nil {
		return "", err
	}
	return GetHostByCode(appName, codeInt), nil
}
