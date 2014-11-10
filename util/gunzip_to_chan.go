package util

/*
import (
"io"
"compress/gzip"
)

type gzipDecompressorDatum struct {
	data []byte
	err error
}

// Fields used only by the decompressor go routine.
type gzipDecompressor struct {
	// Fields used by both reader and writer.
	full chan<- []gzipDecompressorDatum
	empty <-chan []gzipDecompressorDatum

	gzipReader *gzip.Reader
	fileReader io.Reader
}

func (p *gzipDecompressor) Decompress() {
	for {
		// Get a buffer in to which to decompress the file.
		datum, ok := <-empty
		if !ok {
			break
		}

		// Reset datum
		datum.err = nil
		datum.data = data[0:cap(data)]

		n, err := gzipReader.



	}
}


type gzipDecompressorReader struct {
	// Fields used by both reader and writer.
	full chan []byte
	empty chan []byte
	errors chan error

	// Fields used only by the writer.
	gzipReader *gzip.Reader

	// Fields used only by the reader (must be single thread).

	err error
}


func (p *gzipDecompressorReader) Read(p []byte) (n int, err error) {
	select

	if p.err != nil {
		return 0, p.err
	}

}
func (p *gzipDecompressor) Close() error {
}

func fetchAndFill(


func DecompressGzippedFile(filePath string) io.Reader {
	// Open file
	// Create gzip.Reader
	// Create channels
	// Start go routine decompressing file.


}


*/
