package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/knakk/ftx/index"
	"github.com/knakk/intset"
	"github.com/rcrowley/go-tigertonic"
)

var apiMux *tigertonic.TrieServeMux

type PersonRequest struct {
	Name       string
	Department int
	Email      string
	Img        string
	Role       string
	Info       string
	Phone      string
}

type PersonResponse struct {
	ID   int
	Data json.RawMessage
}

type SeveralItemsResponse struct {
	Count  int
	TimeMs float64
	Hits   json.RawMessage
}

func srAsIntSet(sr *index.SearchResults) intset.IntSet {
	s := intset.New()
	for _, h := range sr.Hits {
		s.Add(h.ID)
	}
	return s
}

func init() {
	setupAPIRouting()
}

func setupAPIRouting() {
	apiMux = tigertonic.NewTrieServeMux()
	apiMux.Handle(
		"GET",
		"/person",
		tigertonic.Marshaled(searchPerson))
	apiMux.Handle(
		"GET",
		"/person/{id}",
		tigertonic.Marshaled(getPerson))
	apiMux.Handle(
		"POST",
		"/person",
		tigertonic.Marshaled(createPerson))
	apiMux.Handle(
		"PATCH",
		"/person/{id}",
		tigertonic.Marshaled(updatePerson))
	apiMux.HandleFunc(
		"DELETE",
		"/person/{id}",
		deletePerson)
}

// POST /person
func createPerson(u *url.URL, h http.Header, rq *PersonRequest) (int, http.Header, *PersonResponse, error) {
	if rq.Department == 0 || rq.Name == "" || rq.Email == "" {
		return http.StatusBadRequest, nil, nil, errors.New("required parameters: name, department, email")
	}
	if _, ok := mapDepartments[rq.Department]; !ok {
		return http.StatusBadRequest, nil, nil, errors.New("department doesn't exist")
	}
	img := rq.Img
	if img == "" {
		img = "dummy.png"
	}
	p := PersonRequest{Name: rq.Name, Department: rq.Department, Email: rq.Email, Img: img}
	b, err := json.Marshal(p)
	if err != nil {
		return http.StatusInternalServerError, nil, nil, errors.New("failed to marshal JSON")
	}
	id := persons.Create(&b)
	person, err := persons.Get(id)
	if err != nil {
		log.Println(err)
		return http.StatusInternalServerError, nil, nil, errors.New("failed to save person to database")
	}

	folkSaver.Inc()
	// index the person:
	go func() {
		analyzer.Index(fmt.Sprintf("%v %v", rq.Name, mapDepartments[rq.Department].Name), id)
	}()

	return http.StatusCreated, http.Header{
		"Content-Location": {fmt.Sprintf(
			"%s://%s/api/person/%s",
			u.Scheme,
			u.Host,
			id,
		)},
	}, &PersonResponse{id, *person}, nil
}

// PATCH /person/{id}
func updatePerson(u *url.URL, h http.Header, rq *PersonRequest) (int, http.Header, *PersonResponse, error) {
	full := u.Query().Get("full")
	idStr := u.Query().Get("id")
	var p, oldp PersonRequest
	id, err := strconv.Atoi(idStr)
	if err != nil {
		return http.StatusBadRequest, nil, nil, errors.New("person ID must be an integer")
	}
	oldperson, err := persons.Get(id)
	if err != nil {
		return http.StatusNotFound, nil, nil, errors.New("person not found")
	}
	err = json.Unmarshal(*oldperson, &oldp)
	if err != nil {
		log.Println(err)
		return http.StatusInternalServerError, nil, nil, errors.New("failed to store in database")
	}
	if _, ok := mapDepartments[rq.Department]; !ok {
		return http.StatusBadRequest, nil, nil, errors.New("department doesn't exist")
	}
	if full == "yes" {
		p = *rq
	} else {
		p = PersonRequest{
			Name: rq.Name, Department: rq.Department, Email: rq.Email, Img: rq.Img,
			Info: oldp.Info, Role: oldp.Role, Phone: oldp.Phone}
	}
	b, err := json.Marshal(p)
	if err != nil {
		return http.StatusInternalServerError, nil, nil, errors.New("failed to marshal JSON")
	}
	persons.Set(id, &b)
	newperson, err := persons.Get(id)
	if err != nil {
		return http.StatusInternalServerError, nil, nil, errors.New("failed to store in database")
	}

	folkSaver.Inc()
	go func() {
		// 1. unindex old person:
		analyzer.UnIndex(fmt.Sprintf("%v %v %v %v",
			oldp.Name, mapDepartments[oldp.Department].Name, oldp.Role, oldp.Info), id)
		// 2. index new person:
		analyzer.Index(fmt.Sprintf("%v %v %v %v",
			p.Name, mapDepartments[p.Department].Name, p.Role, p.Info), id)

	}()

	return http.StatusOK, nil, &PersonResponse{id, *newperson}, nil
}

// GET /person/{id}
func getPerson(u *url.URL, h http.Header, _ interface{}) (int, http.Header, *PersonResponse, error) {
	idStr := u.Query().Get("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		return http.StatusBadRequest, nil, nil, errors.New("person ID must be an integer")
	}
	person, err := persons.Get(id)
	if err != nil {
		return http.StatusNotFound, nil, nil, errors.New("person not found")
	}
	return http.StatusOK, nil, &PersonResponse{id, *person}, nil
}

// DELETE /person/{id}
func deletePerson(w http.ResponseWriter, r *http.Request) {
	idStr := strings.TrimPrefix(r.URL.Path, "/person/")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "person ID must be an integer", http.StatusBadRequest)
		return
	}
	_, err = persons.Get(id)
	if err != nil {
		http.Error(w, "person not found", http.StatusBadRequest)
	}
	persons.Del(id)
	folkSaver.Inc()
	fmt.Fprint(w, "OK")
}

// GET /person?q="searchterm" or /person?page=x
func searchPerson(u *url.URL, h http.Header, _ interface{}) (int, http.Header, *SeveralItemsResponse, error) {
	// fetch persons for admin listing
	page := u.Query().Get("page")
	if page != "" {
		t0 := time.Now()
		all := persons.all.ToSlice()
		sort.Sort(sort.Reverse(sort.IntSlice(all)))
		max := 20
		if len(all) < 20 {
			max = len(all)
		}
		return http.StatusOK, nil, &SeveralItemsResponse{
				Count:  len(all[0:max]),
				TimeMs: float64(time.Now().Sub(t0)) / 1000,
				Hits:   persons.GetSeveral(all[0:max])},
			nil
	}

	// search
	q := u.Query().Get("q")
	if q == "" {
		return http.StatusBadRequest, nil, nil, errors.New("search query missing (q)")
	}
	t0 := time.Now()
	parsedQuery := strings.Split(strings.ToLower(q), " ") // TODO Query Parser
	query := index.NewQuery().Must(parsedQuery)
	res := analyzer.Idx.Query(query)
	hits := srAsIntSet(res)
	hitsPersons := persons.GetSeveral(hits.ToSlice())
	return http.StatusOK, nil, &SeveralItemsResponse{
			Count:  hits.Size(),
			TimeMs: float64(time.Now().Sub(t0)) / 1000,
			Hits:   hitsPersons},
		nil
}
