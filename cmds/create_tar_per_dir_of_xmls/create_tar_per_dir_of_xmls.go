package main

import (
	"archive/tar"
	//	"archive/zip"
	"bufio"
	"compress/gzip"
	"flag"
	"fmt"
	//"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"
)

func Exists(name string) bool {
	fi, err := os.Stat(name)
	return err == nil && fi != nil
}

// If the name of the file is <NNNNN>.xml (NNNN = Unix Epoch milliseconds),
// convert it to <HHMMSS>.<mmm>.xml (time of day in Hours (24), Minutes,
// Seconds, and milliseconds).
var unix_millis_name_re *regexp.Regexp

func TimeFileName(name string) string {
	if unix_millis_name_re == nil {
		unix_millis_name_re = regexp.MustCompile("^(\\d{10})(\\d{3})\\.xml$")
	}
	submatches := unix_millis_name_re.FindStringSubmatch(name)
	if submatches == nil {
		return name
	}
	epoch_seconds, _ := strconv.ParseInt(submatches[1], 10, 64)
	t := time.Unix(epoch_seconds, 0)
	return fmt.Sprintf("%02d%02d%02d.%v.xml",
		t.Hour(),
		t.Minute(),
		t.Second(),
		submatches[2])
}

// If the name (without extension) of the file is <YYYYMMDD> (i.e. a date)
// convert it to <YYYY>-<MM>-<DD>.
var date_name_re *regexp.Regexp

func DateFileName(name string) string {
	if date_name_re == nil {
		date_name_re = regexp.MustCompile("^(\\d{4})(\\d{2})(\\d{2})$")
	}
	submatches := date_name_re.FindStringSubmatch(name)
	if submatches == nil {
		return name
	}
	return fmt.Sprintf("%s-%s-%s", submatches[1], submatches[2], submatches[3])
}

func TarXmlDir(dirname string, tar_path string) {
	// Open file for writing.
	outf, err := os.Create(tar_path)
	if err != nil {
		log.Printf("Error creating archive file: %v", err)
		return
	}
	defer outf.Close()
	log.Println("Opened file for writing: ", tar_path)

	bw := bufio.NewWriterSize(outf, 64*1024)
	defer bw.Flush()

	gzip_writer := gzip.NewWriter(bw)
	defer gzip_writer.Close()

	archive_writer := tar.NewWriter(gzip_writer)
	defer archive_writer.Close()

	files, err := ioutil.ReadDir(dirname)
	for _, elem := range files {
		if !strings.HasSuffix(elem.Name(), ".xml") {
			continue
		}
		in_path := filepath.Join(dirname, elem.Name())
		b, err := ioutil.ReadFile(in_path)
		if err != nil {
			log.Printf("Error: %v", err)
			continue
		}
		out_name := TimeFileName(elem.Name())
		fi, err := os.Stat(in_path)
		hdr := &tar.Header{
			Name:    out_name,
			Size:    int64(len(b)),
			Mode:    int64(fi.Mode() & os.ModePerm),
			ModTime: fi.ModTime(),
		}
		if err := archive_writer.WriteHeader(hdr); err != nil {
			log.Printf("Error: %v", err)
			continue
		}
		if _, err := archive_writer.Write(b); err != nil {
			log.Printf("Error: %v", err)
		}
		//		if elem.Name() != out_name {
		//			log.Printf("Added to archive: %s  (was %s)", out_name, elem.Name())
		//		} else {
		//			log.Printf("Added to archive: %s", out_name)
		//		}
	}
	if err := archive_writer.Close(); err != nil {
		log.Printf("Error: %v", err)
	}
}

func exec(root_dir string, exclude map[string]bool, output_dir string) {
	files, err := ioutil.ReadDir(root_dir)
	if err != nil {
		log.Fatal(err)
		return
	}
	for _, elem := range files {
		if !elem.IsDir() {
			continue
		}
		if exclude[elem.Name()] {
			continue
		}
		input_dir := filepath.Join(root_dir, elem.Name())

		tar_path := filepath.Join(
			output_dir, DateFileName(elem.Name())+".tar.gz")
		if Exists(tar_path) {
			//log.Println("Already exists: ", tar_path)
			continue
		}
		TarXmlDir(input_dir, tar_path)
	}
}

var from_dir = flag.String(
	"from", "X:\\mbta\\locations\\raw",
	"Parent directory of daily directories of raw locations")
var to_dir = flag.String(
	"to", "C:\\temp\\raw-mbta-locations",
	"Directory into which to write gzipped tar files, one per daily directory")

func main() {
	fmt.Printf("os.Args: %#v\n", os.Args)

	max := runtime.GOMAXPROCS(-1)
	fmt.Println("Original GOMAXPROCS:", max)
	cpus := runtime.NumCPU()
	fmt.Println("NumCPU:", cpus)
	max = runtime.GOMAXPROCS(cpus)
	max = runtime.GOMAXPROCS(-1)
	fmt.Println("Current GOMAXPROCS:", max)

	//	time.Sleep(5 * time.Minute)

	flag.Parse()
	exclude := make(map[string]bool)
	now := time.Now()
	exclude[now.Format("2006-01-02")] = true
	exclude[now.Format("20060102")] = true
	fmt.Printf("Excluding: %v\n\n", exclude)

	exec(*from_dir, exclude, *to_dir)
}
