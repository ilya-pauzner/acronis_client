package main

import (
	"crypto/md5"
	"gorm.io/gorm/utils/tests"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"sort"
	"testing"
)

func md5SumOfFile(path string) ([]byte, error) {
	f1, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	defer f1.Close()

	f1hash := md5.New()
	_, err = io.Copy(f1hash, f1)
	if err != nil {
		return nil, err
	}

	return f1hash.Sum(nil), nil
}

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

	for _, pair := range []struct {
		path1 string
		path2 string
	}{
		{
			path1: "input/2",
			path2: "output/2",
		},
		{
			path1: "input/4",
			path2: "output/4",
		},
	} {
		f1sum, err := md5SumOfFile(pair.path1)
		if err != nil {
			log.Fatal(err)
		}

		f2sum, err := md5SumOfFile(pair.path2)
		if err != nil {
			log.Fatal(err)
		}

		tests.AssertEqual(t, f1sum, f2sum)
	}

}
