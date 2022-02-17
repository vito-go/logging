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

	"github.com/gin-gonic/gin"

	"github.com/vito-go/mylog"

	"github.com/vito-go/logging"
)

//go:embed unilog_tid.html
// appNameList
var unilogTidHtml string

//  appNameList 未来考虑走配置
var appNameList []string

// getLogInfoNameFunc 默认的日志文件名称规则.
var getLogInfoNameFunc = func(app string) (logInfo, logErr string) {
	return app + ".log", app + "-err.log"
}

// tidUnilogGet 分布式日志搜索路由入口. 打开tid搜索界面或者进行跳转. 与logging滚动查看日志(即 logClient )进行了解藕
// Deprecated: 由logging导航接管此功能.
func tidUnilogGet(ctx *gin.Context) {
	// 优先判断是否有app 和 log 参数，可以进行跳转
	app := ctx.Query("app")     // err-im  或者im
	logName := ctx.Query("log") // err-im  或者im
	if logName == "" {
		logName = app + ".log" // 默认日志名称
	}
	if host := chooseOneHostByAppName(app); host != "" {
		if redirect(ctx.Writer, ctx.Request, host, app, logName) {
			return
		}
	}
	b, _ := json.Marshal(appNameList)
	// 替换符加个单引号防止被格式化
	ctx.Writer.Header().Set("Cache-Control", "no-cache") // 必须设置无缓存，不然跳转到以前的ip。
	strings.NewReplacer("'{{appNameList}}'", string(b), "'{{BasePath}}'", logging.BasePath)
	ctx.Writer.WriteString(strings.ReplaceAll(unilogTidHtml, "'{{appNameList}}'", string(b)))
	return
}

// tidUnilogPost post 请求获取日志.
func tidUnilogPost(ctx *gin.Context) {
	tidStr := ctx.PostForm("tid")
	appName := ctx.PostForm("appName")
	tidInt, err := strconv.ParseInt(tidStr, 10, 64)
	if err != nil || tidInt <= 0 {
		mylog.Ctx(ctx).WithField("addr", ctx.Request.RemoteAddr).WithField("tid", tidStr).Warnf(
			"查看tid日志失败！tid错误")
		ctx.Writer.WriteHeader(http.StatusForbidden)
		return
	}
	host, err := getIpByTidStr(appName, tidStr)
	if err != nil {
		ctx.Writer.WriteString(err.Error())
		return
	}
	if host == "" {
		ctx.Writer.WriteString("未找到对应集群节点")
		return
	}
	tidURL := fmt.Sprintf("http://%s%s/%s/tid-search", host, logging.BasePath, appName)
	postForm := url.Values{}
	postForm.Set("tid", tidStr)
	mylog.Ctx(ctx).WithFields("tidURL", tidURL, "postForm", postForm).Info(ctx.Request.RemoteAddr, "搜索日志")
	response, err := http.PostForm(tidURL, postForm)
	if err != nil {
		mylog.Ctx(ctx).WithFields("tidURL", tidURL, "postForm", postForm).Error(err)
		ctx.Writer.WriteString(err.Error())
		return
	}
	defer response.Body.Close()
	io.Copy(ctx.Writer, response.Body)

}

// tidUniAPPLog 分布式日志搜索路由入口. 与logging滚动查看日志(即 logClient )进行了解藕
func tidUniAPPLog(ctx *gin.Context) {
	app := ctx.Param("app")
	if host := chooseOneHostByAppName(app); host != "" {
		// 可以选择redirect 跳转 或者reverse反向代理，未来可以考虑走配置
		if reverse(ctx.Writer, ctx.Request, host) {
			return
		}
	}
	b, _ := json.Marshal(appNameList)
	// 替换符加个单引号防止被格式化
	ctx.Writer.Header().Set("Cache-Control", "no-cache") // 必须设置无缓存，不然跳转到以前的ip。
	ctx.Writer.WriteString(strings.ReplaceAll(unilogTidHtml, "'{{appNameList}}'", string(b)))
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
