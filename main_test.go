package main

import (
	"gorm.io/gorm/utils/tests"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"sort"
	"testing"
)

func Test(t *testing.T) {
	srv := httptest.NewServer(http.FileServer(http.Dir("input")))
	defer srv.Close()

	err := os.RemoveAll("output")
	if err != nil {
		log.Fatal(err)
	}
	err = os.Mkdir("output", os.ModeDir)
	if err != nil {
		log.Fatal(err)
	}

	Download(srv.URL, "output")

	dir, err := os.Open("output")
	if err != nil {
		log.Fatal(err)
	}
	defer dir.Close()

	files, err := dir.Readdirnames(0)
	if err != nil {
		log.Fatal(err)
	}

	sort.Strings(files)
	tests.AssertEqual(t, files, []string{"2", "4"})
}
