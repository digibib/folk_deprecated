package main

import (
	"flag"
	"html/template"
	"log"
	"net/http"

	"github.com/rcrowley/go-tigertonic"
)

var (
	templates = template.Must(template.ParseFiles("data/html/folk.html", "data/html/admin.html"))
	mux       *tigertonic.TrieServeMux
)

func mainHandler(w http.ResponseWriter, r *http.Request) {
	err := templates.ExecuteTemplate(w, "folk.html", nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func adminHandler(w http.ResponseWriter, r *http.Request) {
	err := templates.ExecuteTemplate(w, "admin.html", nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// serveFile serves a single file from disk.
func serveFile(filename string) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filename)
	}
}

func init() {
	mux = tigertonic.NewTrieServeMux()
	mux.HandleFunc(
		"GET",
		"/",
		mainHandler)
	mux.HandleFunc(
		"GET",
		"/admin",
		adminHandler)
	mux.HandleFunc(
		"GET",
		"/robots.txt",
		serveFile("data/robots.txt"))
	mux.HandleFunc(
		"GET",
		"/css/styles.css",
		serveFile("data/css/styles.css"))
	mux.HandleNamespace("/data/img", http.FileServer(http.Dir("data/img/")))
}

func main() {
	port := flag.String("port", "9999", "serve from this port")
	flag.Parse()

	server := tigertonic.NewServer(":"+*port, mux)
	log.Fatal(server.ListenAndServe())
}
