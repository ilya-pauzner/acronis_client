package main

import (
	"crypto/md5"
	"gorm.io/gorm/utils/tests"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
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

func getSortedFilesList(dir string) ([]string, error) {
	output, err := os.Open(dir)
	if err != nil {
		return nil, err
	}
	defer output.Close()

	files, err := output.Readdirnames(0)
	if err != nil {
		return nil, err
	}

	sort.Strings(files)

	return files, nil
}

func testDirectory(t *testing.T, prefix string) error {
	inputDir := path.Join(prefix, "input")
	outputDir := path.Join(prefix, "output")
	etalonDir := path.Join(prefix, "etalon")

	srv := httptest.NewServer(http.FileServer(http.Dir(inputDir)))
	defer srv.Close()

	err := os.RemoveAll(outputDir)
	if err != nil {
		return err
	}
	err = os.Mkdir(outputDir, os.ModeDir)
	if err != nil {
		return err
	}

	Download(srv.URL, outputDir)

	etalonFiles, err := getSortedFilesList(etalonDir)
	if err != nil {
		return err
	}
	outputFiles, err := getSortedFilesList(outputDir)
	if err != nil {
		return err
	}
	tests.AssertEqual(t, etalonFiles, outputFiles)

	for _, name := range etalonFiles {
		path1 := path.Join(etalonDir, name)
		f1sum, err := md5SumOfFile(path1)
		if err != nil {
			return err
		}

		path2 := path.Join(outputDir, name)
		f2sum, err := md5SumOfFile(path2)
		if err != nil {
			return err
		}

		tests.AssertEqual(t, f1sum, f2sum)
	}
	return nil
}

func Test1(t *testing.T) {
	if err := testDirectory(t, path.Join("tests", "1")); err != nil {
		log.Fatal(err)
	}
}

func Test2(t *testing.T) {
	if err := testDirectory(t, path.Join("tests", "2")); err != nil {
		log.Fatal(err)
	}
}

func Test3(t *testing.T) {
	if err := testDirectory(t, path.Join("tests", "3")); err != nil {
		log.Fatal(err)
	}
}
