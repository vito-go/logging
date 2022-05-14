package router

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type Router interface {
	Route(method string, path string, h func(w http.ResponseWriter, r *http.Request))
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

func (g *Gin) Route(method string, path string, f func(w http.ResponseWriter, r *http.Request)) {
	if method == HttpMethodAny {
		g.engine.Any(path, func(ctx *gin.Context) { f(ctx.Writer, ctx.Request) })
		return
	}
	g.engine.Handle(method, path, func(ctx *gin.Context) { f(ctx.Writer, ctx.Request) })
}

type ServeMux struct {
	mux *http.ServeMux
}

func NewServeMux(mux *http.ServeMux) *ServeMux {
	return &ServeMux{mux: mux}
}

func (s *ServeMux) Route(method string, path string, h func(w http.ResponseWriter, r *http.Request)) {
	s.mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
		if method != HttpMethodAny {
			if r.Method != method {
				http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
				return
			}
		}
		h(w, r)
	})
}
