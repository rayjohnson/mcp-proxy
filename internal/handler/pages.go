package handler

import (
	"html/template"
	"net/http"
	"path/filepath"
)

var templates *template.Template

// InitTemplates loads all HTML templates from the given directory.
func InitTemplates(dir string) error {
	pattern := filepath.Join(dir, "**", "*.html")
	tmpl, err := template.ParseGlob(pattern)
	if err != nil {
		// Fallback: try root-level templates only.
		pattern = filepath.Join(dir, "*.html")
		tmpl, err = template.ParseGlob(pattern)
		if err != nil {
			return err
		}
	}
	templates = tmpl
	return nil
}

func renderTemplate(w http.ResponseWriter, name string, data any) {
	if templates == nil {
		http.Error(w, "templates not initialized", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := templates.ExecuteTemplate(w, name, data); err != nil {
		http.Error(w, "template error", http.StatusInternalServerError)
	}
}

// LoginPage serves the login form.
func LoginPage(w http.ResponseWriter, r *http.Request) {
	renderTemplate(w, "layout", nil)
}

// RegisterPage serves the registration form.
func RegisterPage(w http.ResponseWriter, r *http.Request) {
	renderTemplate(w, "layout", nil)
}

// DashboardPage serves the developer dashboard.
func DashboardPage(w http.ResponseWriter, r *http.Request) {
	renderTemplate(w, "layout", nil)
}
