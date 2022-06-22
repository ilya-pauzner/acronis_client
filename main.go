package main

import (
	"bytes"
	"flag"
	"gopkg.in/xmlpath.v2"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"
	"sort"
	"sync"
)

const chunkSize = 1024

type Record struct {
	filepath string
	indexOfA int
}

func IndexOfCharInFile(filepath string, ch byte) int {
	buffer := make([]byte, chunkSize)

	f, err := os.Open(filepath)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	length := 0
	for {
		bytesread, err := f.Read(buffer)
		if err != nil {
			if err != io.EOF {
				log.Fatal(err)
			}
			break
		}
		if idx := bytes.IndexByte(buffer[:bytesread], ch); idx != -1 {
			return length + idx
		}
		length += bytesread
	}
	return -1
}

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

func Worker(filename string, serverUrl string, dstPath string) {
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

	log.Printf("Downloaded contents of %s to location %s", fullUrl, fullPath)
}

func Download(serverUrl string, dstPath string) {
	log.Printf("Will download files from URL %s to folder %s", serverUrl, dstPath)
	resp, err := http.Get(serverUrl)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()

	root, err := xmlpath.ParseHTML(resp.Body)
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("Got HTML page from %s", serverUrl)

	xpath := xmlpath.MustCompile("/html/body/pre/a")

	var wg sync.WaitGroup
	var filepaths = make([]string, 0)

	for iter := xpath.Iter(root); iter.Next(); {
		filename := iter.Node().String()
		filepaths = append(filepaths, path.Join(dstPath, filename))
		wg.Add(1)
		go func() {
			defer wg.Done()
			Worker(filename, serverUrl, dstPath)
		}()
	}
	wg.Wait()

	records := make([]Record, 0)
	for _, filepath := range filepaths {
		records = append(records, Record{
			filepath: filepath,
			indexOfA: IndexOfCharInFile(filepath, 'A'),
		})
	}

	sort.Slice(records, func(i, j int) bool {
		return records[i].indexOfA < records[j].indexOfA
	})

	if len(records) > 0 && records[len(records)-1].indexOfA > -1 {
		minIdx := -1
		for _, record := range records {
			if record.indexOfA > -1 {
				minIdx = record.indexOfA
				break
			}
		}
		log.Printf("Earliest position where 'A' occured in all files is %d", minIdx)
		for _, record := range records {
			if record.indexOfA != minIdx {
				err = os.Remove(record.filepath)
				log.Printf("%s deleted due to having 'A' too late or not having at all", record.filepath)
				if err != nil {
					log.Fatal(err)
				}
			}
		}
	} else {
		for _, record := range records {
			err = os.Remove(record.filepath)
			log.Printf("%s deleted due to having 'A' too late or not having at all", record.filepath)
			if err != nil {
				log.Fatal(err)
			}
		}
	}
}

func main() {
	serverUrlPtr := flag.String("url", "http://localhost:8080", "url to file server")
	dstPathPtr := flag.String("dst", "dst", "path to directory")
	flag.Parse()

	Download(*serverUrlPtr, *dstPathPtr)
}
