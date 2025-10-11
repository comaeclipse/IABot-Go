package handler

import (
    "embed"
    "html/template"
    "net/http"
)

//go:embed templates/index.html
var tmplFS embed.FS

// Handler serves the minimal interface page for Vercel Serverless Functions.
func Handler(w http.ResponseWriter, r *http.Request) {
    t, err := template.ParseFS(tmplFS, "templates/index.html")
    if err != nil {
        http.Error(w, "template error", http.StatusInternalServerError)
        return
    }
    _ = t.Execute(w, map[string]any{
        "Title":   "IABot-Go",
        "Message": "Hello from the minimal IABot-Go interface page (Vercel).",
    })
}

