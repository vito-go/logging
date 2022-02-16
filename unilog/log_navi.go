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
	appHosts := GetAllAppHosts()
	logApps := make([]LogApp, 0, 10)
	for app, hosts := range appHosts {
		logInfo, logErr := getLogInfoNameFunc(app)
		for _, host := range hosts {
			logApps = append(logApps, LogApp{
				BasePath: logging.BasePath,
				App:      app,
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
