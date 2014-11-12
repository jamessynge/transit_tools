package main

import (
	"archive/zip"
	"fmt"
	"log"
)

func main() {
	r, err := zip.OpenReader("c:/temp/raw-mbta-locations/20130517.zip")
	if err != nil {
		log.Fatal(err)
	}
	defer r.Close()
	for _, f := range r.File {
		fmt.Printf("Zip contains file: %s\n", f.Name)
	}
}
