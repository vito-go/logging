package router

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/vito-go/mylog"
)

type Router interface {
	Route(path string, h func(w http.ResponseWriter, r *http.Request), methods ...string)
}

const (
	HttpMethodAny = `HttpMethodAny`
)

type Gin struct {
	engine *gin.Engine
}

func NewGin(engine *gin.Engine) *Gin {
	return &Gin{engine: engine}
}

func (g *Gin) Route(path string, f func(w http.ResponseWriter, r *http.Request), methods ...string) {
	ctx := context.Background()
	if len(methods) == 0 {
		mylog.Ctx(ctx).WithFields("method", "GET", "path", path).Info("Gin register router")
		g.engine.Handle(http.MethodGet, path, func(ctx *gin.Context) { f(ctx.Writer, ctx.Request) })
		return
	}
	for _, method := range methods {
		if method == HttpMethodAny {
			mylog.Ctx(ctx).WithFields("method", HttpMethodAny, "path", path).Info("Gin register router")
			g.engine.Any(path, func(ctx *gin.Context) { f(ctx.Writer, ctx.Request) })
			return
		}
		mylog.Ctx(ctx).WithFields("method", method, "path", path).Info("Gin register router")
		g.engine.Handle(method, path, func(ctx *gin.Context) { f(ctx.Writer, ctx.Request) })
	}

}

type ServeMux struct {
	mux *http.ServeMux
}

func NewServeMux(mux *http.ServeMux) *ServeMux {
	return &ServeMux{mux: mux}
}

func (s *ServeMux) Route(path string, h func(w http.ResponseWriter, r *http.Request), methods ...string) {
	ctx := context.Background()
	mylog.Ctx(ctx).WithFields("methods", methods, "path", path).Info("ServeMux register router")
	s.mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
		var t bool
		for _, method := range methods {
			if method == HttpMethodAny {
				t = true
				break
			}
			if r.Method == method {
				// 必须命中一个方法
				t = true
				break
			}
		}
		if !t {
			http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
			return
		}
		h(w, r)
	})
}
