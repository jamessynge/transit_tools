// Rename files and dirs whose name is <YYYYMMDD><rest> to <YYYY-MM-DD><rest>.
package main

import (
	"archive/tar"
	"archive/zip"
	"bufio"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"regexp"
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
	name = filepath.Base(name)
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

type FileHeaderAndContents struct {
	zip.FileHeader
	Contents []byte
}

func ReadZipContentsToChannel(zip_path string,
	c chan<- *FileHeaderAndContents) {
	defer close(c)
	// Open a zip archive for reading.
	r, err := zip.OpenReader(zip_path)
	if err != nil {
		log.Fatal(err)
	}
	defer r.Close()
	log.Printf("Reading %v", zip_path)
	for _, f := range r.File {
		rc, err := f.Open()
		defer rc.Close()
		if err != nil {
			log.Fatal(err)
		}
		if true {
			buf := bytes.NewBuffer(make([]byte, 0, f.UncompressedSize64+1))
			n, err := buf.ReadFrom(rc)
			if err != nil {
				log.Fatal(err)
			}
			if uint64(n) != f.UncompressedSize64 {
				log.Fatalf("Expected to read %v bytes, not %v, for zip entry %v",
					f.UncompressedSize64, n, f.Name)
			}
			c <- &FileHeaderAndContents{
				FileHeader: f.FileHeader,
				Contents:   buf.Bytes(),
			}
		} else {
			contents, err := ioutil.ReadAll(rc)
			if err != nil {
				log.Fatal(err)
			}
			if uint64(len(contents)) != f.UncompressedSize64 {
				log.Fatalf("Expected to read %v bytes, not %v, for zip entry %v",
					f.UncompressedSize64, len(contents), f.Name)
			}
			c <- &FileHeaderAndContents{
				FileHeader: f.FileHeader,
				Contents:   contents,
			}
		}
	}
}

type TarHeaderAndContents struct {
	tar.Header
	Contents []byte
}

func WriteChannelToTarWriter(
	w *tar.Writer, c <-chan *TarHeaderAndContents) {
	for {
		item, ok := <-c
		if !ok {
			return
		}
		if item == nil {
			log.Fatal("Channel contained nil")
			continue
		}
		if err := w.WriteHeader(&item.Header); err != nil {
			log.Fatalf("Error: %v\nHeader: %v", err, item.Header)
			continue
		}
		if _, err := w.Write(item.Contents); err != nil {
			log.Fatalf("Error: %v\n", err)
			continue
		}
		//		log.Printf("Added to archive: %s", item.Name)
	}
}

func WriteChannelToTarPath(
	tar_path string, c <-chan *TarHeaderAndContents, done chan<- bool) {
	defer close(done)
	// Open file for writing.
	f, err := os.Create(tar_path)
	if err != nil {
		log.Fatalf("Error creating archive file: %v", err)
	}
	defer f.Close()
	log.Printf("Writing %v", tar_path)
	bw := bufio.NewWriterSize(f, 64*1024)
	defer bw.Flush()
	var w io.Writer = bw
	if strings.HasSuffix(tar_path, ".tar.gz") ||
		strings.HasSuffix(tar_path, ".tgz") {
		gw := gzip.NewWriter(w)
		defer gw.Close()
		w = gw
	}
	tw := tar.NewWriter(w)
	defer tw.Close()
	WriteChannelToTarWriter(tw, c)
}

func ConvertZipToTar(zip_path, tar_path string) {
	in_c := make(chan *FileHeaderAndContents, 5)
	out_c := make(chan *TarHeaderAndContents, 5)
	done_c := make(chan bool)
	go ReadZipContentsToChannel(zip_path, in_c)
	go WriteChannelToTarPath(tar_path, out_c, done_c)
	for {
		in, ok := <-in_c
		if !ok {
			break
		}
		out_name := TimeFileName(in.Name)
		out := &TarHeaderAndContents{
			Header: tar.Header{
				Name:    out_name,
				Size:    int64(len(in.Contents)),
				Mode:    int64(in.Mode() & os.ModePerm),
				ModTime: in.ModTime(),
			},
			Contents: in.Contents,
		}
		out_c <- out
	}
	close(out_c)
	// Need to wait for the writer to finish writing.
	<-done_c
	//log.Printf("Done writing %v", tar_path)
}

// If the name (without extension) of the file is <YYYYMMDD> (i.e. a date)
// convert it to <YYYY>-<MM>-<DD>.
var date_name_re *regexp.Regexp

func DateFileName(name string) string {
	if date_name_re == nil {
		date_name_re = regexp.MustCompile("^(\\d{4})(\\d{2})(\\d{2})$")
	}
	name = filepath.Base(name)
	submatches := date_name_re.FindStringSubmatch(name)
	if submatches == nil {
		return name
	}
	return fmt.Sprintf("%s-%s-%s", submatches[1], submatches[2], submatches[3])
}

func processZip(dirname, zip_name string) {
	start_time := time.Now()
	zip_path := filepath.Join(dirname, zip_name)
	base_name := DateFileName(string(zip_name[0 : len(zip_name)-4]))
	tar_path := filepath.Join(dirname, base_name+".tar.gz")
	if Exists(tar_path) {
		log.Printf("Already exists: %v", tar_path)
		return
	}
	temp_tar_path := filepath.Join(dirname, "NEW_"+base_name+".tar.gz")
	if Exists(temp_tar_path) {
		err := os.Remove(temp_tar_path)
		if err != nil {
			log.Fatalf("Unable to remove temp file: %v", err)
		}
	}
	ConvertZipToTar(zip_path, temp_tar_path)
	err := os.Rename(temp_tar_path, tar_path)
	if err != nil {
		log.Fatalf("Unable to rename new file: %v", err)
	}
	log.Printf("Renamed to %v", tar_path)
	end_time := time.Now()
	dur := end_time.Sub(start_time)
	log.Printf("Zip to Tar duration: %v", dur)
}

func exec(dirname string) {
	name_re := regexp.MustCompile("^(20\\d{2})(\\d{2})(\\d{2})(\\..*)?$")
	files, err := ioutil.ReadDir(dirname)
	if err != nil {
		log.Fatal(err)
		return
	}
	for _, elem := range files {
		name := elem.Name()
		submatches := name_re.FindStringSubmatch(name)
		if submatches == nil {
			continue
		}
		new_name := fmt.Sprintf("%s-%s-%s%s",
			submatches[1], submatches[2], submatches[3], submatches[4])
		new_path := filepath.Join(dirname, new_name)
		i := 0
		for Exists(new_path) {
			fmt.Printf("Already exists: %s\n", new_path)
			i += 1
			new_name = fmt.Sprintf("%s-%s-%s (%d)%s",
				submatches[1], submatches[2], submatches[3], i, submatches[4])
			new_path = filepath.Join(dirname, new_name)
		}
		old_path := filepath.Join(dirname, name)
		fmt.Printf("Renaming %s\n", old_path)
		fmt.Printf("      to %s\n", new_path)
		err := os.Rename(old_path, new_path)
		if err != nil {
			log.Fatal(err)
			return
		}
	}
}

func main() {
	exec("c:/temp/raw-mbta-locations")
}
