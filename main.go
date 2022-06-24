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
	"sync"
)

const chunkSize = 64 * 1024
const notFound = int64(math.MaxInt64)

func failIfNotNil(err error) {
	if err != nil {
		log.Fatal(err)
	}
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

func Step(name string, bodies *map[string]io.ReadCloser, files *map[string]io.WriteCloser, portions, lengths, positionOfA *sync.Map) {
	limited := io.LimitReader((*bodies)[name], chunkSize)
	buf, err := io.ReadAll(limited)
	failIfNotNil(err)

	written, err := io.Copy((*files)[name], bytes.NewReader(buf))
	failIfNotNil(err)

	portions.Store(name, written)

	length, ok := lengths.Load(name)
	if !ok {
		log.Fatalf("No file %s in lengths map for some reason", name)
	}

	idx := bytes.IndexByte(buf, 'A')
	if idx != -1 {
		positionOfA.Store(name, length.(int64)+int64(idx))
	}

	lengths.Store(name, length.(int64)+written)
}

func CleanAndClose(name string, bodies *map[string]io.ReadCloser, files *map[string]io.WriteCloser) error {
	err := (*bodies)[name].Close()
	if err != nil {
		return err
	}
	log.Printf("Stopped downloading file %s", name)
	delete(*bodies, name)

	err = (*files)[name].Close()
	if err != nil {
		return err
	}
	log.Printf("Closed for writing file %s", name)
	delete(*files, name)

	return nil
}

func RemoveFile(name string, dstPath string) error {
	err := os.Remove(path.Join(dstPath, name))
	if err != nil {
		return err
	}
	log.Printf("Removed file %s", name)
	return nil
}

func GatherFilenames(serverUrl string) ([]string, error) {
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

func Download(serverUrl string, dstPath string) {
	log.Printf("Will download files from URL %s to folder %s", serverUrl, dstPath)

	filenames, err := GatherFilenames(serverUrl)
	failIfNotNil(err)

	var bodies = make(map[string]io.ReadCloser)
	var files = make(map[string]io.WriteCloser)
	var lengths sync.Map // filename -> total bytes read (>=0)

	for _, filename := range filenames {
		fullUrl, err := JoinPath(serverUrl, filename)
		failIfNotNil(err)
		resp, err := http.Get(fullUrl)
		failIfNotNil(err)
		bodies[filename] = resp.Body

		fullPath := path.Join(dstPath, filename)
		f, err := os.Create(fullPath)
		failIfNotNil(err)
		files[filename] = f

		lengths.Store(filename, int64(0))
	}

	var wg sync.WaitGroup
	var positionOfA sync.Map // filename -> position (>=0)
	var portions sync.Map    // filename -> portion size
	position := notFound

	for len(files) > 0 {
		for name := range files {
			wg.Add(1)
			nameCopy := name
			go func() {
				defer wg.Done()
				Step(nameCopy, &bodies, &files, &portions, &lengths, &positionOfA)
			}()
		}
		wg.Wait()

		// not yet found A
		if position == notFound {
			for name := range files {
				maybePosition, ok := positionOfA.Load(name)
				if ok {
					if position > maybePosition.(int64) {
						position = maybePosition.(int64)
					}
				}
			}

			if position != notFound {
				log.Printf("Earliest position of A was found at %d", position)

				// Stop writing all those with position more or without position altogether
				blockList := make([]string, 0)
				for name := range files {
					maybePosition, ok := positionOfA.Load(name)
					if !ok || maybePosition.(int64) > position {
						blockList = append(blockList, name)
					}
				}

				// Closing and removing
				for _, name := range blockList {
					log.Printf("File %s has no As, or an A too late", name)
					failIfNotNil(CleanAndClose(name, &bodies, &files))
					failIfNotNil(RemoveFile(name, dstPath))
				}
			}
		}

		blockList := make([]string, 0)
		for name := range files {
			portion, ok := portions.Load(name)
			if !ok {
				log.Fatalf("No file %s in portions map for some reason", name)
			}

			if portion.(int64) != int64(chunkSize) {
				// because of LimitedReader + ReadAll trick, it means that file has ended
				blockList = append(blockList, name)
			}
		}

		// Closing and maybe removing
		for _, name := range blockList {
			log.Printf("File %s has ended", name)
			failIfNotNil(CleanAndClose(name, &bodies, &files))
			_, ok := positionOfA.Load(name)
			if !ok {
				failIfNotNil(RemoveFile(name, dstPath))
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
