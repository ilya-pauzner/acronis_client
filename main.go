package main

import (
	"bytes"
	"flag"
	"gopkg.in/xmlpath.v2"
	"io"
	"log"
	"math"
	"net/http"
	"net/url"
	"os"
	"path"
	"time"
)

// set to 0 in release mode
const debug = 1
const chunkSize = int64(debug)*2 + int64(1-debug)*64*1024
const sleepDuration = debug * 10 * time.Second

const notFound = int64(math.MaxInt64)

// Message sent by workers to main
type Message struct {
	filename    string
	finished    bool
	positionOfA int64
	length      int64
}

func failIfNotNil(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func joinUrls(baseUrl string, relativeUrl string) (string, error) {
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

func removeFile(name string, dstPath string) error {
	err := os.Remove(path.Join(dstPath, name))
	if err != nil {
		return err
	}
	log.Printf("Removed file %s", name)
	return nil
}

func gatherFilenames(serverUrl string) ([]string, error) {
	resp, err := http.Get(serverUrl)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	root, err := xmlpath.ParseHTML(resp.Body)
	if err != nil {
		return nil, err
	}

	log.Printf("Got HTML page from %s", serverUrl)

	xpath := xmlpath.MustCompile("/html/body/pre/a")

	var filenames = make([]string, 0)
	for iter := xpath.Iter(root); iter.Next(); {
		filename := iter.Node().String()
		filenames = append(filenames, filename)
	}

	return filenames, nil
}

func maybeTerminate(name string, done map[string]chan struct{}, deleteList map[string]bool) {
	if !deleteList[name] {
		close(done[name])
		deleteList[name] = true
	}
}

func worker(name string, serverUrl string, dstPath string, ch chan Message, done chan struct{}) {
	log.Printf("Starting download of file %s", name)

	fullUrl, err := joinUrls(serverUrl, name)
	failIfNotNil(err)
	resp, err := http.Get(fullUrl)
	failIfNotNil(err)
	defer resp.Body.Close()

	fullPath := path.Join(dstPath, name)
	f, err := os.Create(fullPath)
	failIfNotNil(err)
	defer f.Close()

	length := int64(0)
	positionOfA := notFound
	for loop := true; loop; {
		select {
		case <-done:
			log.Printf("Download of file %s prematurely terminated", name)
			loop = false
		default:
			rdr := io.LimitReader(resp.Body, chunkSize)

			chunk, err := io.ReadAll(rdr)
			failIfNotNil(err)
			time.Sleep(sleepDuration)
			written, err := io.Copy(f, bytes.NewReader(chunk))
			failIfNotNil(err)

			if positionOfA == notFound {
				idx := bytes.IndexByte(chunk, 'A')
				if idx != -1 {
					positionOfA = length + int64(idx)
				}
			}

			length += written

			ch <- Message{
				filename:    name,
				finished:    false,
				positionOfA: positionOfA,
				length:      length,
			}

			if written < chunkSize {
				loop = false
			}
		}
	}
	ch <- Message{
		filename:    name,
		finished:    true,
		positionOfA: positionOfA,
		length:      length,
	}
	log.Printf("Download of file %s finished", name)
}

func Download(serverUrl string, dstPath string) {
	log.Printf("Will download files from URL %s to folder %s", serverUrl, dstPath)

	filenames, err := gatherFilenames(serverUrl)
	failIfNotNil(err)

	ch := make(chan Message, 2*len(filenames))

	done := make(map[string]chan struct{})
	for _, filename := range filenames {
		done[filename] = make(chan struct{})
		go worker(filename, serverUrl, dstPath, ch, done[filename])
	}

	deleteList := make(map[string]bool)
	positionOfAs := make(map[string]int64)
	lengths := make(map[string]int64)

	finished := 0
	positionOfA := notFound
	for finished < len(filenames) {
		msg := <-ch
		positionOfAs[msg.filename] = msg.positionOfA
		lengths[msg.filename] = msg.length

		if positionOfA > msg.positionOfA {
			positionOfA = msg.positionOfA

			for _, filename := range filenames {
				maybeLength, okLength := lengths[filename]
				maybePosition, okPosition := positionOfAs[filename]

				if okLength && okPosition && maybeLength > positionOfA && maybePosition > positionOfA {
					maybeTerminate(filename, done, deleteList)
				}
			}
		}

		if msg.length > positionOfA && msg.positionOfA > positionOfA {
			maybeTerminate(msg.filename, done, deleteList)
		}

		if msg.finished {
			if msg.positionOfA == notFound {
				maybeTerminate(msg.filename, done, deleteList)
			}
			finished += 1
		}
	}

	for name := range deleteList {
		failIfNotNil(removeFile(name, dstPath))
	}
}

func main() {
	serverUrlPtr := flag.String("url", "http://localhost:8080", "url to file server")
	dstPathPtr := flag.String("dst", "dst", "path to directory")
	flag.Parse()

	Download(*serverUrlPtr, *dstPathPtr)
}
