package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"regexp"

	"github.com/gorilla/securecookie"
	"github.com/gorilla/sessions"
	"github.com/rcrowley/go-tigertonic"
)

const MAX_MEM_SIZE = 2 * 1024 * 1024 // 2 MB

var (
	templates = template.Must(template.ParseFiles(
		"data/html/folk.html",
		"data/html/admin.html",
		"data/html/login.html"))
	mux                  *tigertonic.TrieServeMux
	departments, persons *DB
	err                  error
	store                *sessions.CookieStore
	username, password   *string
	imageFileNames       = regexp.MustCompile(`(\.png|\.jpg|\.jpeg)$`)
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

type person struct {
	ID    int
	Name  string
	Role  string
	Dept  int
	Email string
	Img   string
	Phone string
	Info  string
}

func deptHierarchy(db *DB) []depts {
	var (
		r   []depts
		d   dept
		max = db.Size()
	)
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

// Handlers:

func mainHandler(w http.ResponseWriter, r *http.Request) {
	data := struct {
		Departments []depts
	}{
		deptHierarchy(departments),
	}
	err := templates.ExecuteTemplate(w, "folk.html", data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func adminHandler(w http.ResponseWriter, r *http.Request) {
	// session, err := store.Get(r, "folke_sjef")
	// if err != nil {
	// 	// cookie found, but couldn't decode it
	// 	log.Printf("%v", err)
	// }
	// if session.IsNew {
	// 	loginHandler(w, r)
	// 	return
	// }
	var imageFiles []string
	files, err := ioutil.ReadDir("./data/img/")
	if err == nil {
		for _, f := range files {
			if imageFileNames.MatchString(f.Name()) {
				imageFiles = append(imageFiles, f.Name())
			}
		}
	}

	data := struct {
		Departments []depts
		Images      []string
	}{
		deptHierarchy(departments),
		imageFiles,
	}
	err = templates.ExecuteTemplate(w, "admin.html", data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func loginHandler(w http.ResponseWriter, r *http.Request) {
	err = templates.ExecuteTemplate(w, "login.html", nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func authHandler(w http.ResponseWriter, r *http.Request) {
	u := r.FormValue("username")
	p := r.FormValue("password")
	if u == *username && p == *password {
		session := createSession(r)
		err := session.Save(r, w)
		if err != nil {
			log.Printf("%v", err)
		}
		fmt.Fprint(w, "OK")
		return
	}
	http.Error(w, "feil brukernavn eller passord", http.StatusUnauthorized)
}

func createSession(r *http.Request) *sessions.Session {
	session, err := store.Get(r, "folke_sjef")
	if err != nil {
		log.Printf("%v", err)
	}
	if session.IsNew {
		session.Options.Path = "/admin"
		session.Options.MaxAge = 0
		session.Options.HttpOnly = false
		session.Options.Secure = true
	}
	return session
}

func uploadHandler(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(MAX_MEM_SIZE); err != nil {
		log.Println(err)
		http.Error(w, err.Error(), http.StatusForbidden)
	}

	for key, value := range r.MultipartForm.Value {
		fmt.Fprintf(w, "%s:%s ", key, value)
		log.Printf("%s:%s", key, value)
	}

	for _, fileHeaders := range r.MultipartForm.File {
		for _, fileHeader := range fileHeaders {
			file, _ := fileHeader.Open()
			path := fmt.Sprintf("data/img/%s", fileHeader.Filename)
			buf, _ := ioutil.ReadAll(file)
			ioutil.WriteFile(path, buf, os.ModePerm)
		}
	}
}

// serveFile serves a single file from disk.
func serveFile(filename string) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filename)
	}
}

func init() {
	// Load DBs or create new if they don't exist
	departments, err = NewFromFile("data/avd.db")
	if err != nil {
		departments = New(32)
	}
	persons, err = NewFromFile("data/folk.db")
	if err != nil {
		persons = New(256)
	}

	// HTTP routing
	mux = tigertonic.NewTrieServeMux()
	mux.HandleFunc(
		"POST",
		"/upload",
		uploadHandler)
	mux.HandleFunc(
		"GET",
		"/",
		mainHandler)
	mux.HandleFunc(
		"POST",
		"/authenticate",
		authHandler)
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

	setupAPIRouting() // apiMux
	mux.HandleNamespace("/api", apiMux)

	tigertonic.SnakeCaseHTTPEquivErrors = true
}

func main() {
	port := flag.String("port", "9999", "serve from this port")
	username = flag.String("u", "admin", "admin username")
	password = flag.String("p", "secret", "admin password")

	flag.Parse()

	store = sessions.NewCookieStore(securecookie.GenerateRandomKey(32), securecookie.GenerateRandomKey(32))

	server := tigertonic.NewServer(":"+*port, mux)
	log.Fatal(server.ListenAndServe())
}
