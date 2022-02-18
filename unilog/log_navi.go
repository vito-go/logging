package unilog

import (
	_ "embed"
	"html/template"
	"strings"

	"github.com/gin-gonic/gin"
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

type logNavi struct {
	getLogNameByApp LogInfoNameFunc
}

// LoggingNavi  log导航
func (l *logNavi) LoggingNavi(ctx *gin.Context) {
	appHostList := GetAllAppHosts()
	logApps := make([]LogApp, 0, len(appHostList))
	for _, s := range appHostList {
		logInfo, logErr := l.getLogNameByApp(s.App)
		for _, host := range s.Hosts {
			logApps = append(logApps, LogApp{
				BasePath: _basePath,
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
