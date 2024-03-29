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
	"sync"

	"github.com/gorilla/securecookie"
	"github.com/gorilla/sessions"
	"github.com/knakk/ftx"
	"github.com/rcrowley/go-tigertonic"

	//"github.com/davecheney/profile"
)

const MAX_MEM_SIZE = 2 * 1024 * 1024 // 2 MB

var (
	templates = template.Must(template.ParseFiles(
		"data/html/folk.html",
		"data/html/admin.html",
		"data/html/login.html"))
	mux                *tigertonic.TrieServeMux
	persons            *DB
	departments        []depts
	mapDepartments     = make(map[int]dept)
	err                error
	store              *sessions.CookieStore
	username, password *string
	imageFileNames     = regexp.MustCompile(`(\.png|\.jpg|\.jpeg)$`)
	folkSaver          *saver
	analyzer           *ftx.Analyzer
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

type dbPerson struct {
	ID   int
	Data person
}

type allPersons []dbPerson

type person struct {
	ID         int
	Name       string
	Role       string
	Department int
	Email      string
	Img        string
	Phone      string
	Info       string
}

// saver saves the db after X edits has ben made
type saver struct {
	sync.Mutex
	db    *DB
	file  string
	count int
	max   int
}

func (s *saver) Inc() {
	s.Lock()
	defer s.Unlock()
	s.count++
	if s.count == s.max {
		log.Printf("Saving db: %s", s.file)
		err := s.db.Dump(s.file)
		if err != nil {
			log.Println(err)
		}
		s.count = 0
	}
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
		departments,
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
		NumFolks    int
	}{
		departments,
		imageFiles,
		persons.Size(),
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

// uploadHandler upload image files to the folder /data/img/
func uploadHandler(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(MAX_MEM_SIZE); err != nil {
		log.Println(err)
		http.Error(w, err.Error(), http.StatusForbidden)
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

// indexDB indexes all searchable fields in the person database.
func indexDB(db *DB, a *ftx.Analyzer) {
	all := db.All()
	var allp allPersons
	err = json.Unmarshal(all, &allp)
	if err != nil {
		log.Println(err)
		return
	}
	for _, p := range allp {
		//fmt.Printf("%v: %v %v\n", p.ID, p.Data.Name, mapDepartments[p.Data.Department].Name)
		a.Index(fmt.Sprintf("%v %v %v %v",
			p.Data.Name, mapDepartments[p.Data.Department].Name, p.Data.Role, p.Data.Info), p.ID)
	}
}

func init() {
	// Search Analyzer & index
	analyzer = ftx.NewNGramAnalyzer(1, 20)

	// load department db
	deptsdb, err := NewFromFile("data/avd.db")
	if err == nil {
		departments = deptHierarchy(deptsdb)
		for _, d := range departments {
			mapDepartments[d.ID] = dept{d.ID, d.Name, d.Parent}
			for _, dd := range d.Depts {
				mapDepartments[dd.ID] = dept{dd.ID, dd.Name, dd.Parent}
			}
		}
	}

	// Load person DB or create new if it doesn't exist
	persons, err = NewFromFile("data/folk.db")
	if err != nil {
		log.Println(err)
		persons = New(256)
	}
	indexDB(persons, analyzer)

	// Save DB to disk every 15 edits
	folkSaver = &saver{db: persons, file: "./data/folk.db", max: 15}

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
	//defer profile.Start(profile.CPUProfile).Stop()
	port := flag.String("port", "9999", "serve from this port")
	username = flag.String("u", "admin", "admin username")
	password = flag.String("p", "secret", "admin password")

	flag.Parse()

	store = sessions.NewCookieStore(securecookie.GenerateRandomKey(32), securecookie.GenerateRandomKey(32))

	server := tigertonic.NewServer(":"+*port, mux)
	log.Fatal(server.ListenAndServe())
}
