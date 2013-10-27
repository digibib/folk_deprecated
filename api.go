package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/rcrowley/go-tigertonic"
)

var apiMux *tigertonic.TrieServeMux

type PersonRequest struct {
	Name       string
	Department int
	Email      string
	Img        string
}

type PersonResponse struct {
	ID   int
	Data json.RawMessage
}

type DepartmentRequest struct {
	Name   string
	Parent int
}

type DepartmentResponse struct {
	ID   int
	Data json.RawMessage
}

type SeveralItemsResponse struct {
	Count  int
	TimeMs float64
	Hits   json.RawMessage
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
		"GET",
		"/department",
		tigertonic.Marshaled(getAllDepartments))
	apiMux.Handle(
		"GET",
		"/department/{id}",
		tigertonic.Marshaled(getDepartment))
	apiMux.Handle(
		"POST",
		"/department",
		tigertonic.Marshaled(createDepartment))
}

// POST /department
func createDepartment(u *url.URL, h http.Header, rq *DepartmentRequest) (int, http.Header, *DepartmentResponse, error) {
	if rq.Name == "" {
		return http.StatusBadRequest, nil, nil, errors.New("required parameters: name")
	}
	if rq.Parent != 0 {
		_, err := departments.Get(rq.Parent)
		if err != nil {
			return http.StatusBadRequest, nil, nil, errors.New("parent department doesn't exist")
		}
	}
	p := DepartmentRequest{rq.Name, rq.Parent}
	b, err := json.Marshal(p)
	if err != nil {
		return http.StatusInternalServerError, nil, nil, errors.New("failed to marshal JSON")
	}
	id := departments.Create(&b)
	Department, err := departments.Get(id)
	if err != nil {
		return http.StatusInternalServerError, nil, nil, errors.New("failed to save Department to database")
	}
	return http.StatusCreated, http.Header{
		"Content-Location": {fmt.Sprintf(
			"%s://%s/api/department/%s",
			u.Scheme,
			u.Host,
			id,
		)},
	}, &DepartmentResponse{id, *Department}, nil
}

// POST /person
func createPerson(u *url.URL, h http.Header, rq *PersonRequest) (int, http.Header, *PersonResponse, error) {
	if rq.Department == 0 || rq.Name == "" || rq.Email == "" {
		return http.StatusBadRequest, nil, nil, errors.New("required parameters: name, department, email")
	}
	_, err := departments.Get(rq.Department)
	if err != nil {
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
		return http.StatusInternalServerError, nil, nil, errors.New("failed to save person to database")
	}
	return http.StatusCreated, http.Header{
		"Content-Location": {fmt.Sprintf(
			"%s://%s/api/person/%s",
			u.Scheme,
			u.Host,
			id,
		)},
	}, &PersonResponse{id, *person}, nil
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

// GET /department/{id}
func getDepartment(u *url.URL, h http.Header, _ interface{}) (int, http.Header, *DepartmentResponse, error) {
	idStr := u.Query().Get("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		return http.StatusBadRequest, nil, nil, errors.New("department ID must be an integer")
	}
	dept, err := departments.Get(id)
	if err != nil {
		return http.StatusNotFound, nil, nil, errors.New("department not found")
	}
	return http.StatusOK, nil, &DepartmentResponse{id, *dept}, nil
}

// GET /department
func getAllDepartments(u *url.URL, h http.Header, _ interface{}) (int, http.Header, *SeveralItemsResponse, error) {
	t0 := time.Now()
	return http.StatusOK, nil, &SeveralItemsResponse{
			Count:  departments.Size(),
			TimeMs: float64(time.Now().Sub(t0)) / 1000,
			Hits:   departments.All()},
		nil
}

// GET /person?q="searchterm"
func searchPerson(u *url.URL, h http.Header, _ interface{}) (int, http.Header, *SeveralItemsResponse, error) {
	q := u.Query().Get("q")
	if q == "" {
		return http.StatusBadRequest, nil, nil, errors.New("search query missing (q)")
	}
	t0 := time.Now()
	size, hits := 0, []byte("") //persons.Search(u.Query().Get("q"))
	return http.StatusOK, nil, &SeveralItemsResponse{
			Count:  size,
			TimeMs: float64(time.Now().Sub(t0)) / 1000,
			Hits:   hits},
		nil
}
