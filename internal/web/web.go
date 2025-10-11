package web

import (
    "html/template"
    "net/http"
)

//go:generate echo "no generated assets yet"

// Very simple inline parse of the template from disk. In a real app, use go:embed.
func IndexHandler(w http.ResponseWriter, r *http.Request) {
    if r.URL.Path != "/" {
        http.NotFound(w, r)
        return
    }
    t, err := template.ParseFiles("internal/web/templates/index.html")
    if err != nil {
        http.Error(w, "template error", http.StatusInternalServerError)
        return
    }
    _ = t.Execute(w, map[string]any{
        "Title":   "IABot-Go",
        "Message": "Hello from the minimal IABot-Go interface page.",
    })
}

