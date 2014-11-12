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
	if err == nil && stat_a.ModTime().After(stat_b.ModTime()) {
		log.Printf("%s modified at %s", path_a, stat_a.ModTime().String()) 
		log.Printf("%s modified at %s", path_b, stat_b.ModTime().String()) 
		return true
	}
	return false
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

func GetAllFilesInDir(
	dirname string, fileinfo chan<- os.FileInfo, filebody chan<- []byte) {
	defer close(filebody)
	defer close(fileinfo)
	files, err := ioutil.ReadDir(dirname)
	if err != nil {
		log.Printf("Error processing directory: %s\nErr: %v", dirname, err)
		return
	}

	for _, elem := range files {
		if !strings.HasSuffix(elem.Name(), ".xml") {
			continue
		}
		fileinfo <- elem
		in_path := filepath.Join(dirname, elem.Name())
		b, err := ioutil.ReadFile(in_path)
		if err != nil {
			log.Printf("Error reading file: %s\nErr: %v", in_path, err)
			b = nil
		}
		filebody <- b
	}
}

func TarXmlDir(dirname string, tar_path string) {
	// Start reading the xml files while we get the output ready.
	fileinfo_chan := make(chan os.FileInfo, 50)
	filebody_chan := make(chan []byte, 50)
	closed := false
	go GetAllFilesInDir(dirname, fileinfo_chan, filebody_chan)
	defer func() {
		if !closed {
			close(fileinfo_chan)
			close(filebody_chan)
		}
	}()

	var latest_modtime time.Time
	have_latest_modtime := false

	// Open file for writing. Use a modified name while writing, then change
	// the name after having updated the ModTime.
	temp_file_path := tar_path + ".in_progress"
	if Exists(temp_file_path) {
		os.Remove(temp_file_path)
	}
	outf, err := os.Create(temp_file_path)
	if err != nil {
		log.Printf("Error creating archive file: %s\nErr: %v", temp_file_path, err)
		return
	}
	defer func() {
		if err := outf.Close(); err != nil {
			log.Fatalf("Error closing file writer: %v", err)
		}
		if (have_latest_modtime) {
      currenttime := time.Now().Local()
      // Ignoring errors
			err = os.Chtimes(temp_file_path, currenttime, latest_modtime)
			if err != nil {
				log.Printf("Ignoring error from os.Chtimes: %v", err)
			}
		}
		if len(tar_path) > 0 {
			if Exists(tar_path) {
				os.Remove(tar_path)
			}
			err = os.Rename(temp_file_path, tar_path)
			if err != nil {
				log.Fatalf("Rename error: %v\nOld name: %s\nNew name: %s",
								   err, temp_file_path, tar_path)
			}
			log.Printf("Finished writing: %s", tar_path)
		}
	}()
	log.Println("Opened file for writing: ", temp_file_path)

	bw1 := bufio.NewWriterSize(outf, 64*1024)
	defer bw1.Flush()

	gzip_writer, err := gzip.NewWriterLevel(bw1, gzip.BestCompression)
	if err != nil {
		log.Printf("Error creating gzip compressor: %v", err)
		return
	}
	defer gzip_writer.Close()

	bw2 := bufio.NewWriterSize(gzip_writer, 64*1024)
	defer bw2.Flush()

	archive_writer := tar.NewWriter(bw2)
	defer func() {
		if err := archive_writer.Close(); err != nil {
			log.Fatalf("Error closing tar writer: %v", err)
		}
	}()

	num_files := 0
	num_fileinfo_chan_not_empty := 0
	num_filebody_chan_not_empty := 0
	sum_fileinfo_chan_len := 0
	sum_filebody_chan_len := 0

	for {
		l := len(fileinfo_chan)
		if l > 0 {
			num_fileinfo_chan_not_empty++
			sum_fileinfo_chan_len += l
		}
		elem, ok := <-fileinfo_chan
		if !ok {
			// All done with the xml files.
			closed = true
			have_latest_modtime = false // Seems the file dates are later
																  // than the folder dates!
			break
		}
		num_files++
		out_name := TimeFileName(elem.Name())
		hdr := &tar.Header{
			Name:    out_name,
			Size:    elem.Size(),
			Mode:    int64(elem.Mode() & os.ModePerm),
			ModTime: elem.ModTime(),
		}
		if err := archive_writer.WriteHeader(hdr); err != nil {
			log.Printf("Error writing tar header: %v", err)
			tar_path = "" // Prevent renaming
			break
		}
		l = len(filebody_chan)
		if l > 0 {
			num_filebody_chan_not_empty++
			sum_filebody_chan_len += l
		}
		filebody, ok := <- filebody_chan
		if !ok {
			// Programming bug.
			log.Fatal("filebody_chan closed early!")
		}
		if filebody == nil {
			// We were unable to read the file. Finish off the archive.
			if _, err := archive_writer.Write(make([]byte, 0)); err != nil {
				log.Printf("Error writing tar body: %v", err)
			}
			tar_path = "" // Prevent renaming
			break
		}
		if int64(len(filebody)) != hdr.Size {
			// Programming bug, or a narrow timing window when file has just been
			// created, but not yet flushed/closed by writer.
			log.Fatalf("FileInfo.Size and len(filebody) aren't the same!\n%v != %v",
			           elem.Size(), len(filebody))
	  }
		if _, err := archive_writer.Write(filebody); err != nil {
			log.Printf("Error writing tar body: %v", err)
			tar_path = "" // Prevent renaming
			break
		}
		if !have_latest_modtime || latest_modtime.Before(elem.ModTime()) {
			latest_modtime = elem.ModTime()
			have_latest_modtime = true
		}
		if false {
	  	if elem.Name() != out_name {
				log.Printf("Added to archive: %s  (was %s)", out_name, elem.Name())
			} else {
				log.Printf("Added to archive: %s", out_name)
			}
		}
	}
	log.Printf("num_files: %v", num_files) 
	log.Printf("num_fileinfo_chan_not_empty: %v", num_fileinfo_chan_not_empty)
	log.Printf("num_filebody_chan_not_empty: %v", num_filebody_chan_not_empty)
	log.Printf("sum_fileinfo_chan_len: %v", sum_fileinfo_chan_len)
	log.Printf("sum_filebody_chan_len: %v", sum_filebody_chan_len)
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
	"to", "K:\\nextbus\\mbta\\locations\\tars",
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
