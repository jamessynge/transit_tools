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

// Is path_a modified more recently than path_b?
func IsNewer(path_a, path_b string) bool {
	stat_a, err := os.Stat(path_a)
	if err != nil {
		return false
	}
	stat_b, err := os.Stat(path_b)
	return err == nil && stat_a.ModTime().After(stat_b.ModTime())
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

// Parse a date of form <YYYYMMDD> or <YYYY>-<MM>-<DD>.
var date_re *regexp.Regexp
var dash_date_re *regexp.Regexp

func ParseDate(s string) (yyyy, mm, ss string) {
	if date_re == nil {
		date_re = regexp.MustCompile(`^(\d{4})(\d{2})(\d{2})$`)
		dash_date_re = regexp.MustCompile(`^(\d{4})-(\d{2})-(\d{2})$`)
	}
	submatches := date_re.FindStringSubmatch(s)
	if submatches == nil {
		submatches = dash_date_re.FindStringSubmatch(s)
		if submatches == nil {
			return "", "", ""
		}
	}
	return submatches[1], submatches[2], submatches[3]
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

	gzip_writer, err := gzip.NewWriterLevel(bw, gzip.BestCompression)
	if err != nil {
		log.Printf("Error creating gzip compressor: %v", err)
		return
	}
	defer gzip_writer.Close()

	archive_writer := tar.NewWriter(gzip_writer)
	defer archive_writer.Close()

	files, err := ioutil.ReadDir(dirname)
	for _, elem := range files {
		if !strings.HasSuffix(elem.Name(), ".xml") {
			continue
		}
		log.Printf("elem: %+v", elem)
		in_path := filepath.Join(dirname, elem.Name())
		b, err := ioutil.ReadFile(in_path)
		if err != nil {
			log.Printf("Error: %v", err)
			continue
		}
		out_name := TimeFileName(elem.Name())
		fi, err := os.Stat(in_path)
		log.Printf("fi: %+v", fi)
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
		dirname := elem.Name()
		if exclude[dirname] {
			continue
		}
		yyyy, mm, dd := ParseDate(dirname)
		if yyyy == "" {
			log.Println("Unexpected directory name: ", dirname)
			continue
		}
		input_dir := filepath.Join(root_dir, dirname)
		tar_dir := filepath.Join(output_dir, yyyy, mm)
		tar_name := fmt.Sprintf("%s-%s-%s.tar.gz", yyyy, mm, dd)
		tar_path := filepath.Join(tar_dir, tar_name)
		if Exists(tar_path) {
			// If output exists, but input is newer, then replace output.
			if IsNewer(input_dir, tar_path) {
				log.Println("Replacing older output: ", tar_path)
			} else {
				//log.Println("Already exists: ", tar_path)
				continue
			}
		} else {
			err = os.MkdirAll(tar_dir, 0755)
			if err != nil {
				log.Fatal(err)
			}
		}
		TarXmlDir(input_dir, tar_path)
	}
}

var from_dir = flag.String(
	"from", "C:\\nextbus\\mbta\\locations\\raw",
	"Parent directory of daily directories of raw locations")
var to_dir = flag.String(
	"to", "C:\\nextbus\\mbta\\locations\\tars",
	"Directory into which to write gzipped tar files, one per daily directory")

func main() {
	fmt.Printf("os.Args: %#v\n", os.Args)

	hostname, _ := os.Hostname()
	fmt.Println("hostname", hostname)

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
