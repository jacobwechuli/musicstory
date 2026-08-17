// Package render owns both HTML templates and knows how to execute them.
// go:embed requires the .html files to sit next to this file, which is why
// they live here rather than in public/ — once a page is template-driven
// instead of static, it's backend-owned.
package render

import (
	"embed"
	"html/template"
	"net/http"

	"github.com/jacobwechuli/musicstory/internal/models"
)

//go:embed user-template.html
var userTemplateFS embed.FS

//go:embed profiles-template.html
var profilesTemplateFS embed.FS

var (
	userTmpl     = template.Must(template.ParseFS(userTemplateFS, "user-template.html"))
	profilesTmpl = template.Must(template.ParseFS(profilesTemplateFS, "profiles-template.html"))
)

func User(w http.ResponseWriter, page models.UserPage) error {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	return userTmpl.Execute(w, page)
}

func Profiles(w http.ResponseWriter, page models.ProfilesPage) error {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	return profilesTmpl.Execute(w, page)
}
