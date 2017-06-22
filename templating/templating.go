package templating

import (
	"html/template"
	"net/http"
	"path/filepath"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/render"
)

var funcMap = template.FuncMap{
	"admin": func() bool {
		return false
	},
}

type CustomRender struct {
	Template *template.Template
	Data     interface{}
	Name     string
}

type Render map[string]*template.Template

var _ render.HTMLRender = Render{}

func New() Render {
	return make(Render)
}

func (r Render) Add(name string, tmpl *template.Template) {
	if tmpl == nil {
		panic("template can not be nil")
	}
	if len(name) == 0 {
		panic("template name cannot be empty")
	}
	r[name] = tmpl
}

func (r Render) AddFromFiles(name string, files ...string) *template.Template {
	tname := filepath.Base(files[0])
	t, _ := template.New(tname).Funcs(funcMap).ParseFiles(files...)
	r.Add(name, t)
	return t
}

func (r Render) Instance(name string, data interface{}) render.Render {
	return CustomRender{
		Template: r[name],
		Data:     data,
	}
}

var htmlContentType = []string{"text/html; charset=utf-8"}

func (r CustomRender) Render(w http.ResponseWriter) error {
	header := w.Header()
	if val := header["Content-Type"]; len(val) == 0 {
		header["Content-Type"] = htmlContentType
	}
	cfunc := r.Data.(gin.H)["cfunc"]
	if cfunc != nil {
		zz := cfunc.(template.FuncMap)
		r.Template.Funcs(zz)
	}

	if len(r.Name) == 0 {
		return r.Template.Execute(w, r.Data)
	}
	return r.Template.ExecuteTemplate(w, r.Name, r.Data)
}
