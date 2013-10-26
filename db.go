package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"sync"

	"github.com/knakk/intset"
)

// DB is a simple database backed by a map and a mutex.
type DB struct {
	docs map[int][]byte
	sync.RWMutex
	idMax int           // autoincremented ID
	all   intset.IntSet // keep an index of all doc IDs
}

type doc struct {
	ID   int
	Data json.RawMessage
}

// New returns a new database.
func New(size int) *DB {
	return &DB{
		docs: make(map[int][]byte, size),
		all:  intset.New(),
	}
}

// NewFromFile loads db from file into memory and return it as a new database.
func NewFromFile(fname string) (*DB, error) {
	b, err := ioutil.ReadFile(fname)
	if err != nil {
		return nil, err
	}
	var (
		docs  []doc
		db    = New(len(docs))
		bcopy []byte
	)
	err = json.Unmarshal(b, &docs)
	if err != nil {
		return nil, err
	}

	for _, d := range docs {
		bcopy, err = d.Data.MarshalJSON()
		if err != nil {
			return nil, err
		}
		db.Set(d.ID, &bcopy)
	}
	db.Lock()
	defer db.Unlock()
	return db, nil
}

// Size returns the size of the database
func (db *DB) Size() int {
	db.RLock()
	defer db.RUnlock()
	return db.all.Size()
}

// Create inserts a new document into the database. It returns the id of the
// created document.
func (db *DB) Create(data *[]byte) int {
	db.Lock()
	defer db.Unlock()
	db.idMax++
	db.docs[db.idMax] = *data
	db.all.Add(db.idMax)
	return db.idMax
}

// Get returns a document by a given id.
func (db *DB) Get(id int) (*[]byte, error) {
	db.RLock()
	defer db.RUnlock()
	if b, ok := db.docs[id]; ok {
		return &b, nil
	}
	return nil, errors.New("document not found")
}

// Set updates a document at a given id. The document does not need to exist.
func (db *DB) Set(id int, data *[]byte) {
	db.Lock()
	defer db.Unlock()
	db.docs[id] = *data
	// Make sure ID is in the set. Needed when a DB is loaded from file.
	db.all.Add(id)
	// Always update db.idMax to the highest Id number
	if id > db.idMax {
		db.idMax = id
	}
}

// All retuns all the docs in the database as a JSON array, in the form:
// [{"ID": 1, "Data": {jsonData}},{..},{..}]
func (db *DB) All() []byte {
	var allDocs bytes.Buffer
	allDocs.Write([]byte("["))
	db.RLock()
	defer db.RUnlock()
	size := db.Size()
	i := 0
	for k := range db.all {
		allDocs.Write([]byte(fmt.Sprintf("{\"ID\":%v,\"Data\":", k)))
		allDocs.Write(db.docs[k])
		allDocs.Write([]byte("}"))
		i++
		if i != size { // to avoid a trailing comma after last doc
			allDocs.Write([]byte(","))
		}
	}
	allDocs.Write([]byte("]"))
	return allDocs.Bytes()
}

// Dump dumps the DB into a file.
func (db *DB) Dump(fname string) error {
	f, err := os.Create(fname)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(db.All())
	return err
}

// func (db *DB) GetSeveral(docs Intset) []byte
