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

// DB is a simple database backed by a map and a mutex. Since its intended
// use is for mostly reads and very few writes, its not sharded. Consider
// hashing keys and split the store into buckets if the sinlge lock becomes
// an issue.
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

// Set removes a document. Return false if doc doesn't exist. Otherwise true.
func (db *DB) Del(id int) bool {
	db.Lock()
	defer db.Unlock()
	if _, ok := db.docs[id]; ok {
		delete(db.docs, id)
		db.all.Remove(id)
		return true
	}
	return false
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

// GetSeveral fetches several docs from db, as requested by slice of IDs.
func (db *DB) GetSeveral(docs []int) []byte {
	var sevDocs bytes.Buffer
	if len(docs) == 0 {
		return []byte("null") // JSON for empty array
	}
	sevDocs.Write([]byte("["))
	db.RLock()
	defer db.RUnlock()
	size := len(docs)
	i := 0
	for _, k := range docs {
		if b, ok := db.docs[k]; ok {
			sevDocs.Write([]byte(fmt.Sprintf("{\"ID\":%v,\"Data\":", k)))
			sevDocs.Write(b)
			sevDocs.Write([]byte("}"))
			i++
			if i != size { // to avoid a trailing comma after last doc
				sevDocs.Write([]byte(","))
			}
		}
	}
	sevDocs.Write([]byte("]"))
	return sevDocs.Bytes()
}
