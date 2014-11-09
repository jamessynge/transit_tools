package main 

import (
	"archive/zip"
	"fmt"
	//"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type FindFilesResponse struct {
  dirname string
  fileinfo os.FileInfo
  err error
}


func findZipFiles(dirname string, to chan<- *FindFilesResponse) {
	files, err := ioutil.ReadDir(dirname)
	if err != nil {
		to <- &FindFilesResponse{ dirname, nil, err }
	} else {
		for _, elem := range files {
			if strings.HasSuffix(elem.Name(), ".zip") {
				p := filepath.Join(dirname, elem.Name())
				to <- &FindFilesResponse{ p, elem, nil }
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

func Exists(name string) bool {
	fi, err := os.Stat(name)
	return err == nil && fi != nil
}


// If the name of the file is <NNNNN>.xml (NNNN = Unix Epoch milliseconds),
// convert it to <HHMMSS>.<mmm>.xml (time of day in Hours (24), Minutes,
// Seconds, and milliseconds).
var unix_millis_name_re regexp.Regexp
func TimeFileName(name string) string {
	if unix_millis_name_re == nil {
		unix_millis_name_re = regexp.MustCompile("^(\\d{10})(\\d{3})\\.xml$")
	}
	submatches := unix_millis_name_re.FindStringSubmatch(name)
	if submatches == nil {
		return name
	}
	epoch_seconds, _ := strconv.ParseInt(submatches[0], 10, 64)
	t := time.Unix(epoch_seconds, 0)
	return fmt.Sprintf("%02d%02d%02d.%v.xml",
	                   t.Hour(),
	                   t.Minute(),
	                   t.Second(),
	                   submatches[1]);
}


func ZipXmlDir(dirname string, zip_path string) {
	outf, err := os.Create(zip_path)
	if err != nil {
		log.Fatalf("Create: %v", err)
	}
	zw := zip.NewWriter(outf)

	files, err := ioutil.ReadDir(dirname)
	for _, elem := range files {
		if !strings.HasSuffix(elem.Name(), ".xml") { continue }
		in_path := filepath.Join(dirname, elem.Name())
		inf, err := os.Open(in_path)
		if err != nil {
			log.Errorf("Error: %v", err)
			continue
		}
		defer inf.Close()
		out_name := TimeFileName(elem.Name());
		w, err := zw.Create(out_name)
		
		
		
				p := filepath.Join(dirname, elem.Name())
				to <- &FindFilesResponse{ p, elem, nil }
			}
		}
	
	
	
}

func exec(dirname string) {
	files, err := ioutil.ReadDir(dirname)
	if err != nil {
		log.Fatal(err)
		return
	}
	for _, elem := range files {
		if !elem.IsDir() { continue }
		subdir := filepath.Join(dirname, elem.Name())
		zip_path := strings.TrimRight(subdir, "/\\") + ".zip"
		if Exists(zip_path) {
			fmt.Println("Already exists: ", zip_path)
			return
		}
		ZipXmlDir(subdir, zip_path)
	}
}


func main() {
	exec("c:/temp/raw-mbta-locations")
	


	c := startFindingZipFiles("c:/temp/raw-mbta-locations")
	for {
		x, ok := <-c
		if !ok {
			break
		}
		fmt.Println(x.dirname, "\t", x.fileinfo.Name())
	}
}
