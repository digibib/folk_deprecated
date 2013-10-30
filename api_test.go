package main

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"

	"github.com/knakk/ftx"
	"github.com/knakk/specs"
	//"github.com/rcrowley/go-tigertonic"
)

func TestApiCRUD(t *testing.T) {
	persons = New(512)
	mapDepartments = make(map[int]dept)
	mapDepartments[1] = dept{1, "main", 0}
	mapDepartments[2] = dept{2, "xyz", 1}

	analyzer = ftx.NewStandardAnalyzer()
	s := specs.New(t)

	testServer := httptest.NewServer(apiMux)
	defer testServer.Close()

	var testsPOST = []struct {
		url       string
		body      string
		respCode  int
		bodyMatch string
	}{
		{"/person", "{\"name\": \"Mr. P\"}", 400, "required parameters: name, department, email"},
		{"/person", "{\"department\": 1}", 400, "required parameters: name, department, email"},
		{"/person", "{\"name\": \"Mr. P\", \"email\":\"a@b\", \"department\": 100}", 400, "department doesn't exist"},
		{"/person", "{\"name\": \"Mr. P\", \"email\":\"a@b\", \"department\": 1}", 201, "\"Name\":\"Mr. P\""},
		{"/person", "{\"name\": \"a\", \"department\": 1, \"email\":\"a@b\"}", 201, "\"Name\":\"a\""},
		{"/person", "{\"name\": \"bill\", \"department\": 2, \"email\":\"a@b\"}", 201, "\"Name\":\"bill\""},
		{"/person", "{\"name\": \"Mr. c\", \"department\": 2, \"email\":\"a@b\"}", 201, "\"Name\":\"Mr. c\""},
	}

	for _, tt := range testsPOST {
		resp, err := http.Post(testServer.URL+tt.url, "application/json", bytes.NewBufferString(tt.body))
		s.ExpectNilFatal(err)
		s.Expect(tt.respCode, resp.StatusCode)
		body, err := ioutil.ReadAll(resp.Body)
		s.ExpectNilFatal(err)
		r := regexp.MustCompile(tt.bodyMatch)
		if !r.MatchString(string(body)) {
			t.Errorf("expected response body to match \"%v\"\ngot body:\n\"%v\"", tt.bodyMatch, string(body))
		}
	}

	var testsGET = []struct {
		url       string
		respCode  int
		bodyMatch string
	}{
		{"/person/88", 404, "person not found"},
		{"/person/jabba", 400, "person ID must be an integer"},
	}

	for _, tt := range testsGET {
		resp, err := http.Get(testServer.URL + tt.url)
		s.ExpectNilFatal(err)
		s.Expect(tt.respCode, resp.StatusCode)
		body, err := ioutil.ReadAll(resp.Body)
		s.ExpectNilFatal(err)
		r := regexp.MustCompile(tt.bodyMatch)
		if !r.MatchString(string(body)) {
			t.Errorf("expected response body to match \"%v\"\ngot body:\n\"%v\"", tt.bodyMatch, string(body))
		}
	}

	var testsPATCH = []struct {
		url       string
		body      string
		respCode  int
		bodyMatch string
	}{
		{"/person/1", `{"Name":"Mr. Q", "Department":2}`, 200, `"Name":"Mr. Q"`},
	}

	for _, tt := range testsPATCH {
		req, err := http.NewRequest("PATCH", testServer.URL+tt.url, bytes.NewBufferString(tt.body))
		s.ExpectNilFatal(err)
		req.Header.Add("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(req)
		s.ExpectNilFatal(err)
		s.Expect(tt.respCode, resp.StatusCode)
		body, err := ioutil.ReadAll(resp.Body)
		s.ExpectNilFatal(err)
		r := regexp.MustCompile(tt.bodyMatch)
		if !r.MatchString(string(body)) {
			t.Errorf("expected response body to match \"%v\"\ngot body:\n\"%v\"", tt.bodyMatch, string(body))
		}
	}

	resp, err := http.Get(testServer.URL + "/person?q=Mr")
	s.ExpectNilFatal(err)
	s.Expect(200, resp.StatusCode)
	body, err := ioutil.ReadAll(resp.Body)
	s.ExpectNilFatal(err)
	s.ExpectMatches(string(body), "Mr. Q")
	s.ExpectMatches(string(body), "Mr. c")
	s.ExpectNotMatches(string(body), "bill")
}
