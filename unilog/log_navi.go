package unilog

import (
	_ "embed"
	"html/template"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/vito-go/logging"
)

//go:embed log_navi.gohtml
var helloHtml string

type LogApp struct {
	BasePath string
	App      string
	Host     string
	LogInfo  string
	LogErr   string
}

// LogNavi  log导航
func LogNavi(ctx *gin.Context) {
	appHostList := GetAllAppHosts()
	logApps := make([]LogApp, 0, len(appHostList))
	for _, s := range appHostList {
		logInfo, logErr := getLogInfoNameFunc(s.App)
		for _, host := range s.Hosts {
			logApps = append(logApps, LogApp{
				BasePath: logging.BasePath,
				App:      s.App,
				Host:     host,
				LogInfo:  logInfo,
				LogErr:   logErr,
			})
		}
	}
	tmpl := template.New("tmpl")
	tmpl.Funcs(map[string]interface{}{
		"firstUpper": firstUpper,
	})
	t, err := tmpl.Parse(helloHtml)
	if err != nil {
		ctx.Writer.WriteString(err.Error())
		return
	}
	err = t.Execute(ctx.Writer, logApps)
	if err != nil {
		ctx.Writer.WriteString(err.Error())
		return
	}
}

func firstUpper(s string) string {
	if s == "" {
		return ""
	}
	return strings.ToUpper(s[0:1]) + s[1:]
}
