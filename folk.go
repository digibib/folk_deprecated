package main

import (
	"encoding/json"
	"flag"
	"html/template"
	"log"
	"net/http"

	"github.com/rcrowley/go-tigertonic"
)

var (
	templates = template.Must(template.ParseFiles("data/html/folk.html", "data/html/admin.html"))
	mux       *tigertonic.TrieServeMux
	avd, folk *DB
	err       error
)

type dept struct {
	ID     int
	Name   string
	Parent int
}

type depts struct {
	ID     int
	Name   string
	Parent int
	Depts  []dept
}

func deptHierarchy(db *DB) []depts {
	var r []depts
	var d dept
	var max = db.Size()
	for i := 0; i <= max; i++ {
		data, err := db.Get(i)
		if err != nil {
			continue
		}
		err = json.Unmarshal(*data, &d)
		if err != nil {
			continue
		}
		d.ID = i
		if d.Parent == 0 {
			r = append(r, depts{d.ID, d.Name, d.Parent, make([]dept, 0)})
		} else {
			for j := range r {
				if r[j].ID == d.Parent {
					r[j].Depts = append(r[j].Depts, d)
					break
				}
			}
		}
	}
	return r
}
func mainHandler(w http.ResponseWriter, r *http.Request) {
	data := struct {
		Departments []depts
	}{
		deptHierarchy(avd),
	}
	err := templates.ExecuteTemplate(w, "folk.html", data)
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
	avd, err = NewFromFile("data/avd.db")
	if err != nil {
		avd = New(32)
	}

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
