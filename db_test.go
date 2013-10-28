package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"testing"

	"github.com/knakk/specs"
)

type Book struct {
	Author, Title string
	Issued        int
}

func TestCRUD(t *testing.T) {
	s := specs.New(t)

	// new DB
	db := New(32)
	// DB size
	s.Expect(db.Size(), 0)
	// doc not found
	data, err := db.Get(99)
	s.Expect(err.Error(), "document not found")

	// create doc
	book, err := json.Marshal(Book{"Knut Hamsun", "Sult", 1890})
	s.ExpectNilFatal(err)
	id := db.Create(&book)
	s.Expect(db.Size(), 1)
	s.ExpectNot(id, 0)

	// fetch doc
	data, err = db.Get(id)
	s.ExpectNilFatal(err)
	var b Book
	err = json.Unmarshal(*data, &b)
	s.ExpectNilFatal(err)
	s.Expect(b.Author, "Knut Hamsun")
	s.Expect(b.Title, "Sult")
	s.Expect(b.Issued, 1890)

	// update (set) doc
	book2, err := json.Marshal(Book{"Knut Hamsun", "Pan", 1994})
	s.ExpectNilFatal(err)
	id2 := db.Create(&book2)
	book3, err := json.Marshal(Book{"Knut Hamsun", "Pan", 1894})
	s.ExpectNilFatal(err)
	db.Set(id2, &book3)
	data, err = db.Get(id2)
	s.ExpectNilFatal(err)
	err = json.Unmarshal(*data, &b)
	s.Expect(b.Issued, 1894)

	// delete
	book4, err := json.Marshal(Book{"abc", "xyz", 1999})
	s.ExpectNilFatal(err)
	id3 := db.Create(&book4)
	s.Expect(db.Size(), 3)
	db.Del(id3)
	s.Expect(db.Size(), 2)
	s.Expect(db.Del(id3), false) // allready deleted

	// get all docs
	type all []struct {
		ID   int
		Data Book
	}
	allb := db.All()
	var allj all
	err = json.Unmarshal(allb, &allj)
	s.ExpectNilFatal(err)
	s.Expect(db.Size(), len(allj))
	s.Expect(allj[0].Data.Author, "Knut Hamsun")

	// save db
	err = db.Dump("all.json")
	s.ExpectNilFatal(err)

	// load db
	db2, err := NewFromFile("all.json")
	s.ExpectNilFatal(err)
	s.Expect(db2.Size(), 2)

	err = db2.Dump("cpy.json")
	s.ExpectNilFatal(err)

	// compare the db files to make sure they are equal
	f1, err := os.Open("all.json")
	s.ExpectNilFatal(err)
	defer f1.Close()
	f2, err := os.Open("cpy.json")
	s.ExpectNilFatal(err)
	defer f2.Close()
	h1 := sha256.New()
	h2 := sha256.New()
	_, err = io.Copy(h1, f1)
	s.ExpectNilFatal(err)
	_, err = io.Copy(h2, f2)
	s.ExpectNilFatal(err)
	s.Expect(fmt.Sprintf("% x", h1.Sum(nil)), fmt.Sprintf("% x", h2.Sum(nil)))

	// delete dbs
	err = os.Remove("all.json")
	s.ExpectNilFatal(err)
	err = os.Remove("cpy.json")
	s.ExpectNilFatal(err)

}
