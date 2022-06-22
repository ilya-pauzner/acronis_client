package main

import (
	"flag"
	"gopkg.in/xmlpath.v2"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"
	"sync"
)

func JoinPath(baseUrl string, relativeUrl string) (string, error) {
	baseUrlParsed, err := url.Parse(baseUrl)
	if err != nil {
		return "", err
	}

	resultUrlParsed, err := baseUrlParsed.Parse(relativeUrl)
	if err != nil {
		return "", err
	}
	return resultUrlParsed.String(), nil
}

func worker(filename string, serverUrl string, dstPath string) {
	fullPath := path.Join(dstPath, filename)
	f, err := os.Create(fullPath)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	fullUrl, err := JoinPath(serverUrl, filename)
	if err != nil {
		log.Fatal(err)
	}

	resp, err := http.Get(fullUrl)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()

	_, err = io.Copy(f, resp.Body)
	if err != nil {
		log.Fatal(err)
	}
}

func main() {
	serverUrlPtr := flag.String("url", "http://localhost:8080", "url to file server")
	dstPathPtr := flag.String("dst", ".", "path to directory")
	flag.Parse()

	resp, err := http.Get(*serverUrlPtr)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()

	root, err := xmlpath.ParseHTML(resp.Body)
	if err != nil {
		log.Fatal(err)
	}

	xpath := xmlpath.MustCompile("/html/body/pre/a")

	var wg sync.WaitGroup
	for iter := xpath.Iter(root); iter.Next(); {
		filename := iter.Node().String()
		wg.Add(1)
		go func() {
			defer wg.Done()
			worker(filename, *serverUrlPtr, *dstPathPtr)
		}()
	}
	wg.Wait()
}
