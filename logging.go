package logging

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha1"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/vito-go/mylog"

	"github.com/vito-go/logging/router"
	"github.com/vito-go/logging/tid"
)

// logSizeShow 只展示最近的日志数量.
// 暂定4Mb
const (
	logSizeShow  = 2 << 20                 // 只展示最近2M的内容, 大概就是2000行
	pushInterval = time.Second             // 推送间歇。
	waitBrowser  = time.Millisecond * 300 // 留给浏览器处理渲染的时间
	oneSendLine  = 1000                    // 单次发送1000行(一次发送1行,浏览器不停渲染能卡死.) 每行大约1kb
	// maxScanTokenSize 设置单行缓冲最大4Mb .
	maxScanTokenSize = 4 << 20
)

type LogPath struct {
	FileName, RouterPath string
}

type logClient struct {
	logMap             map[string]*os.File // map[pushPath]*os.File 路由对应的文件句柄 *os.File
	logFileNameMap     map[string]string   // 路由对应的文件 map[routerPath]filename
	logFileNamePushMap map[string]string   // 路由对应的文件 map[filename]pushPath

	loginPath     string   // 登录的路由
	tieSearchPath string   // tid搜索 路由
	token         string   // sha1 password==> if token is empty it's no needed to login which is recommend not.
	tidSearchFile *os.File // 指定tid搜索的文件
}

// Config logging配置
type Config struct {
	APPName     string // APPName 服务名称
	Token       string // Token sha1加密字符串.
	LogInfoPath string // LogInfoPath info级别日志文件路径，info日志应该包括所有等级（warn、err）的日志
	LogErrPath  string // LogInfoPath err级别日志文件路径,有可能包含warn日志
}

var basePathNot = []string{`{`, `}`, `:`, `*`}

// BasePath the root path, it should:
//
// 1. must not be empty;
//
// 2. must begin with /
//
// 3. must not end with /
//
// 4  must not contain one of the basePathNot
type BasePath string

// MustCheckBasePath look at the rule of BasePath.
func MustCheckBasePath(path BasePath) {
	if len(path) == 0 {
		panic("empty path")
	}
	if path[0] != '/' {
		panic("base path must begin with /")
	}
	if path[len(path)-1] == '/' {
		panic("base path must not end with /")
	}
	for _, s := range basePathNot {
		if strings.Contains(string(path), s) {
			panic("base path must not contain " + s)
		}
	}
}

// Init (high-level)
// r can user router.NewGin or router.NewServeMux as the router.Router.
// httpPort is the port of the http server listened.
// unilogAddr can be empty if only used locally.
func Init(r router.Router, httpPort int, path BasePath, unilogAddr string, cfg Config) {

	MustCheckBasePath(path)

	mylog.Ctx(context.TODO()).WithField("cfg", cfg).Info("logging init")
	basePath := filepath.Join(string(path), cfg.APPName)
	loginPath := filepath.ToSlash(filepath.Join(basePath, "login"))
	tidSearchPath := filepath.ToSlash(filepath.Join(basePath, "tid-search"))
	// 注册分布式tid.
	err := tid.Register(cfg.APPName, httpPort, unilogAddr)
	if err != nil {
		mylog.Ctx(context.TODO()).Warn(err.Error())
	}
	var logPaths = []LogPath{
		{FileName: cfg.LogInfoPath, RouterPath: filepath.ToSlash(filepath.Join(basePath, filepath.Base(cfg.LogInfoPath)))},
		{FileName: cfg.LogErrPath, RouterPath: filepath.ToSlash(filepath.Join(basePath, filepath.Base(cfg.LogErrPath)))}}
	err = RegisterRouter(r, cfg.LogInfoPath, loginPath, tidSearchPath, cfg.Token, logPaths...)
	if err != nil {
		mylog.Ctx(context.TODO()).Warn(err.Error())
	}
}

// RegisterRouter like Init but it is low-level.
// logInfoPath 日志路径（包含所有等级的日志, loginPath 日志页面登录路由地址，tidSearchPath 日志搜索页面地址
// token 登录授权码， logPaths: 日志与该日志所对应的路由地址
func RegisterRouter(r router.Router, logInfoPath, loginPath, tidSearchPath, token string, logPaths ...LogPath) error {
	// 开启tid搜索引擎服务
	GoRunTidSearch(logInfoPath)
	if len(logPaths) == 0 {
		return errors.New("there is no logPath. logging register failed")
	}
	var tokenSha1 string
	if token != "" {
		tokenSha1 = getSha1Str(token)
	}
	tidSearchF, err := os.OpenFile(logInfoPath, os.O_CREATE|os.O_RDONLY, 0644)
	if err != nil {
		return err
	}
	lc := logClient{
		logMap:             make(map[string]*os.File),
		logFileNameMap:     make(map[string]string),
		logFileNamePushMap: make(map[string]string),
		token:              tokenSha1,
		loginPath:          loginPath,
		tieSearchPath:      tidSearchPath,
		tidSearchFile:      tidSearchF,
	}
	r.Route(loginPath, lc.login, http.MethodGet, http.MethodPost)         // 登录
	r.Route(tidSearchPath, lc.TidSearch, http.MethodGet, http.MethodPost) // tid搜索 html页面服务

	for _, lp := range logPaths {
		logName, routerPath := lp.FileName, lp.RouterPath
		f, err := os.Open(logName)
		if err != nil {
			mylog.Ctx(context.TODO()).WithField("logName", logName).Error("日志路由注册失败：", err.Error())
			continue
		}
		pushPath := filepath.ToSlash(filepath.Join(routerPath, "push"))
		lc.logMap[pushPath] = f
		lc.logFileNameMap[routerPath] = logName
		lc.logFileNamePushMap[logName] = pushPath

		r.Route(routerPath, lc.logIndex, http.MethodGet)
		r.Route(pushPath, lc.logPush, http.MethodGet)
	}
	return nil
}

func (lc *logClient) logIndex(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	if lc.token != "" && !isLogin(r, cookieKey, lc.token) {
		replacer := strings.NewReplacer("'{{jumpPath}}'", path, "'{{loginPath}}'", lc.loginPath)
		w.Write([]byte(replacer.Replace(loginHtml)))
		return
	}
	fileName := lc.logFileNameMap[path]
	logPushPath := lc.logFileNamePushMap[fileName]
	title := filepath.Base(fileName)
	replacer := strings.NewReplacer("'{{title}}'", title, "'{{logPushPath}}'", logPushPath)
	replacer.WriteString(w, logHtml)
}

// login 登录授权校验.
func (lc *logClient) login(w http.ResponseWriter, r *http.Request) {
	ctx := context.WithValue(r.Context(), "tid", tid.Get())
	if r.Method == http.MethodGet {
		if isLogin(r, cookieKey, lc.token) {
			w.Write([]byte("<h1>阁下已登录!</h1>"))
			return
		}
		r := strings.NewReplacer("'{{jumpPath}}'", lc.loginPath, "'{{loginPath}}'", lc.loginPath)
		w.Write([]byte(r.Replace(loginHtml)))
		return
	}
	tokenStr := r.PostFormValue(cookieKey)
	jumpPath := r.PostFormValue("jumpPath")
	if token := getSha1Str(tokenStr); token == lc.token {
		cookiePath := strings.TrimSuffix(r.URL.Path, "login")
		http.SetCookie(w, &http.Cookie{Path: cookiePath, Name: cookieKey, Value: token, Expires: time.Now().Add(time.Hour * 48)})
		http.Redirect(w, r, jumpPath, http.StatusFound)
		mylog.Ctx(ctx).WithField("remoteAddr", r.RemoteAddr).Info("login successfully!")
		return
	}
	replacer := strings.NewReplacer("'{{jumpPath}}'", jumpPath, "'{{loginPath}}'", lc.loginPath, "'{{tokenFailed}}'", "true")
	replacer.WriteString(w, loginHtml)
	mylog.Ctx(ctx).WithFields("remoteAddr", r.RemoteAddr, cookieKey, tokenStr).Warn("login failed!")
	return
}

const cookieKey = "token"

func isLogin(r *http.Request, cookieKey string, cookieValue string) bool {
	ck, err := r.Cookie(cookieKey)
	if err != nil {
		return false
	}
	if ck.Value == cookieValue {
		return true
	}
	return false
}

// logPush SSE log日志推送.
func (lc *logClient) logPush(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	w.Header().Set("Content-Type", "text/event-stream")
	if lc.token != "" && !isLogin(r, cookieKey, lc.token) {
		w.Write([]byte("data: auth failed!\n\n"))
		return
	}
	path := r.URL.Path
	file := lc.logMap[path]
	flusher, ok := w.(http.Flusher)
	if !ok {
		w.Write([]byte("streaming unsupported"))
		return
	}
	flusher.Flush()

	fileInfo, err := file.Stat()
	if err != nil {
		mylog.Ctx(ctx).Error(err)
		return
	}
	var offset int64
	var b []byte

	if size := fileInfo.Size(); size > logSizeShow {
		offset = size - logSizeShow
	}
	b, err = readAllByOffset(file, offset)
	if err != nil {
		mylog.Ctx(ctx).Error(err)
		return
	}
	if len(b) > 0 {
		err = flushBytes(w, flusher, b)
		if err != nil {
			return
		}
		offset += int64(len(b))
	}
	for {
		select {
		case <-ctx.Done(): // write所返回的err有延迟. 用CloseNotify及时的
			mylog.Ctx(ctx).Warn(ctx.Err())
			return
		default:
		}
		b, err = readAllByOffset(file, offset)
		if err != nil {
			mylog.Ctx(ctx).Error(err)
			return
		}
		if len(b) == 0 {
			time.Sleep(pushInterval)
			continue
		}
		err = flushBytes(w, flusher, b)
		if err != nil {
			mylog.Ctx(ctx).Error(err.Error())
			return
		}
		offset += int64(len(b))
		time.Sleep(waitBrowser)
	}
}

func flushBytes(w http.ResponseWriter, flusher http.Flusher, b []byte) error {

	var err error
	// var oneSend string
	var oSend strings.Builder
	var count int
	// 使用bufio.NewScanner 按行解析，如果一行超过65536( 64 * 1024)
	// 那么scanner.Scan将直接返回false！不再往后解析
	scanner := bufio.NewScanner(bytes.NewReader(b))
	scanner.Buffer(make([]byte, 1<<10), maxScanTokenSize)
	for scanner.Scan() {
		line := scanner.Text()
		oSend.WriteString(line)
		oSend.WriteString("<br>")
		if count < oneSendLine {
			count++
			continue
		}
		_, err = w.Write(sseWithData(oSend.String()))
		if err != nil {
			// 应该不需要日志,可能对方关闭了
			return err
		}
		flusher.Flush()
		oSend.Reset()
		count = 0
	}
	err = scanner.Err()
	if err != nil && err != io.EOF {
		mylog.Ctx(context.TODO()).Errorf("日志扫描解析错误: %s", err.Error())
		return err
	}
	if count > 0 {
		_, err = w.Write(sseWithData(oSend.String()))
		if err != nil {
			// 应该不需要日志,可能对方关闭了
			return err
		}
		flusher.Flush()
	}

	return err
}
func readAllByOffset(f *os.File, offset int64) ([]byte, error) {
	b := make([]byte, 64<<10) // 缓存64
	var result []byte
	for {
		n, err := f.ReadAt(b, offset)
		if err != nil {
			if err == io.EOF {
				result = append(result, b[:n]...)
				break
			} else {
				return nil, err
			}
		}
		result = append(result, b[:n]...)
		offset += int64(n)
	}
	return result, nil
}
func readByOffsetAB(f *os.File, offsetA int64, offsetB int64) ([]byte, error) {
	if offsetB-offsetA < 0 {
		return nil, nil
	}
	b := make([]byte, offsetB-offsetA) // 读取64kb
	var result []byte
	n, err := f.ReadAt(b, offsetA)
	if err != nil && err != io.EOF {
		return nil, err
	}
	result = append(result, b[:n]...)
	return result, nil
}

func getSha1Str(s string) string {
	return fmt.Sprintf("%X", sha1.Sum([]byte(s)))
}
func readFromFile(f *os.File, offset int64, size int64) ([]byte, error) {
	b := make([]byte, size) // 读取640kb
	var result []byte
	n, err := f.ReadAt(b, offset)
	if err != nil && err != io.EOF {
		return nil, err
	}
	result = append(result, b[:n]...)
	return result, nil
}
