package main

import (
    "log"
    "net/http"

    "example.com/iabot-go/internal/web"
)

func main() {
    mux := http.NewServeMux()
    mux.HandleFunc("/", web.IndexHandler)

    addr := ":8081"
    log.Printf("IABot-Go web listening on %s", addr)
    if err := http.ListenAndServe(addr, mux); err != nil {
        log.Fatal(err)
    }
}

