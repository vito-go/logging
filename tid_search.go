package logging

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/vito-go/mylog"
)

// logTidLength 限定对进的tid数量.
const logTidLength = 1 << 20

// tidDelay  延迟2秒采集一次
const tidDelay = time.Second * 2

// _logList 全局tid搜索链表.
var _logList = newLogList(logTidLength)

// tidPattern tid应该是一个16位及以上的数字。毫秒级别的时间戳+3位. 但太
const tidPattern = `\d{16,}`

// GoRunTidSearch tid搜索引擎服务. 这里一定要传logPath而不是和日志相同的文件句柄*os.File
func GoRunTidSearch(logPath string) {
	go func() {
		err := runTidSearch(logPath)
		if err != nil {
			log.Printf("tid search run error. server exit. err=%s\n", err.Error())
		}
	}()
}

// runTidSearch tid搜索引擎服务. 这里一定要传logPath而不是和日志相同的文件句柄*os.File
func runTidSearch(logPath string) error {
	f, err := os.Open(logPath)
	if err != nil {
		return fmt.Errorf("tid搜索服务启动失败! err: %s", err.Error())
	}
	var offset int64
	n := readToLogList(offset, f)
	offset += n
	var b []byte
	for {
		b, err = readAllByOffset(f, offset)
		if err != nil {
			return err
		}
		if len(b) == 0 {
			time.Sleep(tidDelay)
			continue
		}
		readToLogList(offset, bytes.NewReader(b))
		offset += int64(len(b))
	}
}

// readToLogList .
func readToLogList(offset int64, reader io.Reader) (n int64) {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 1<<10), maxScanTokenSize)
	for scanner.Scan() {
		a := offset
		line := scanner.Text()
		offset += int64(len(line)) + 1
		reg := regexp.MustCompile(tidPattern)
		tidStr := reg.FindString(line)
		if len(tidStr) == 0 {
			continue
		}
		tidInt, _ := strconv.ParseInt(tidStr, 10, 64)
		b := offset
		_logList.Insert(tidInt, offsetAB{A: a, B: b})
	}
	if err := scanner.Err(); err != nil {
		mylog.Ctx(context.Background()).Errorf("tid搜索引擎服务发生严重错误,请立即修复! ", err.Error())
	}
	n = offset // 明确n的含义, 代表一共读取了多少个字节
	return n
}

// TidSearch 提供一个包含html页面的tid搜索服务.
func (lc *logClient) TidSearch(w http.ResponseWriter, r *http.Request) {
	method := r.Method
	switch method {
	case http.MethodGet, http.MethodPost:
	default:
		w.WriteHeader(http.StatusForbidden)
		return
	}
	if r.Method != "POST" {
		if lc.token != "" && !isLogin(r, cookieKey, lc.token) {
			replacer := strings.NewReplacer("'{{jumpPath}}'", lc.tieSearchPath, "'{{loginPath}}'", lc.loginPath)
			w.Write([]byte(replacer.Replace(loginHtml)))
			return
		}
		w.Write([]byte(strings.ReplaceAll(tidHtml, "{{tieSearchPath}}", lc.tieSearchPath)))
		return
	}

	// post 请求获取日志
	err := r.ParseMultipartForm(2 << 20)
	if err != nil {
		w.WriteHeader(http.StatusForbidden)
		return
	}
	tidStr := r.FormValue("tid")
	tidInt, err := strconv.ParseInt(tidStr, 10, 64)
	if err != nil || tidInt <= 0 {
		w.WriteHeader(http.StatusForbidden)
		return
	}

	var nd *node
	var nodeExist bool

	nd, nodeExist = _logList.Find(tidInt)
	if !nodeExist {
		w.Write([]byte(`<h1>no result</h1>`))
		return
	}
	var result []string
	abS := nd.OffsetABs()
	for _, ab := range abS {
		bb, err := readByOffsetAB(lc.tidSearchFile, ab.A, ab.B)
		if err != nil {
			w.Write([]byte(fmt.Sprintf("readByOffsetAB tid=%d offset=%+v  error: %s", tidInt, ab, err.Error())))
			return
		}
		result = append(result, string(bb))
	}
	b, _ := json.Marshal(result)
	w.Write(b)
}
