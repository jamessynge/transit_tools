package main

import (
	//	"archive/zip"
	"fmt"
	//"io"
	"io/ioutil"
	//	"log"
	"os"
	"path/filepath"
	"strings"
)

type FindFilesResponse struct {
	dirname  string
	fileinfo os.FileInfo
	err      error
}

func findZipFiles(dirname string, to chan<- *FindFilesResponse) {
	files, err := ioutil.ReadDir(dirname)
	if err != nil {
		to <- &FindFilesResponse{dirname, nil, err}
	} else {
		for _, elem := range files {
			if strings.HasSuffix(elem.Name(), ".zip") {
				p := filepath.Join(dirname, elem.Name())
				to <- &FindFilesResponse{p, elem, nil}
			}
		}
	}
	close(to)
}

func startFindingZipFiles(dirname string) <-chan *FindFilesResponse {
	c := make(chan *FindFilesResponse)
	go findZipFiles(dirname, chan<- *FindFilesResponse(c))
	return c
}

func main() {
	c := startFindingZipFiles("c:/temp/raw-mbta-locations")
	for {
		x, ok := <-c
		if !ok {
			break
		}
		fmt.Println(x.dirname, "\t", x.fileinfo.Name())
	}
}
