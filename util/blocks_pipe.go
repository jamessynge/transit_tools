package util

import (
	"io"
	"sync"

	"github.com/golang/glog"
)

// Reads all of an io.Reader into a sequence of blocks, send those blocks
// to a consumer, which returns the empty (consumed) blocks back, limiting
// the memory and read-ahead to that specified when the BlocksPipe was created.
type BlocksPipe struct {
	// Producer goroutine fills buffers and inserts them into the data channel. If
	// it encounters an error, it sets the err field, and closes the data channel.
	// When it is done it closes data without writing to err.
	data chan []byte

	// Consumer must return buffers via the empty channel.
	empty chan []byte

	// First error encountered by producer, which stops the pipe.
	err error

	// Consumer's method of signaling that the producer should stop by writing
	// to or closing this channel. Consumer will no longer access after
	// closing.
	stop chan interface{}
}

// Creates a BlocksPipe reading into blockCount buffers of size blockSize from
// in. Starts a go-routine to read ahead into those blocks. If in is an
// io.ReaderCloser, then in is closed when the consumer closes the
// BlocksPipeReader that wraps the pipe.
func ReadIntoBlocksPipe(in io.Reader, blockSize, blockCount int) *BlocksPipe {
	result := &BlocksPipe{
		data: make(chan []byte, blockCount),
		empty: make(chan []byte, blockCount),
		stop: make(chan interface{}),
		err: nil,
	}
	go func() {
		data := result.data
		empty := result.empty
		stop := result.stop
		defer func() {
			glog.V(1).Infof("BlocksPipe @ %p producer cleaning up...", result)
			if closer, ok := in.(io.Closer); ok {
				if err := closer.Close(); err != nil {
					glog.V(1).Infof("BlocksPipe @ %p in.Close produced error: %v", result, err)
				}
			}
			if stop == nil {
				// Consumer is not going to access BlocksPipe any more.
				result.data = nil
				result.empty = nil
				result.stop = nil
			}
//			glog.V(1).Infof("BlocksPipe @ %p producer done", result)
		}()
		// Create the blocks and "store" them in empty.
		for n := 0; n < blockCount; n++ {
			empty <- make([]byte, blockSize)
		}
//		glog.V(1).Infof("BlocksPipe @ %p starting with %d buffers of %d bytes", result, blockCount, blockSize)
		for {
			// Get a buffer to put data into, but also check for the consumer having
			// signalled for us to stop.
			var buf []byte
			select {
			case buf = <- empty:
				// Got a buffer to work with.
//				glog.V(2).Infof("BlocksPipe @ %p got empty buffer", result)
			case _, _ = <- stop:
				// Consumer wants us to stop (and has stopped reading from
				// the BlocksPipe object).
				glog.V(1).Infof("BlocksPipe @ %p got stop signal instead of receiving empty", result)
				stop = nil
				return
			}
			// Resize the buffer to full size.
			buf = buf[0:cap(buf)]
			// Fill the buffer (to the extent possible).
			size, err := in.Read(buf)
			// Queue the data for the consumer to read, and also check for a stop
			// signal.
			if size > 0 {
//				glog.V(1).Infof("BlocksPipe @ %p in.Read filled buffer with %d bytes", result, size)
				select {
				case data <- buf[0:size]:
				case _, _ = <- stop:
					// Consumer wants us to stop (and has stopped reading from
					// the BlocksPipe object).
					glog.V(1).Infof("BlocksPipe @ %p got stop signal instead of sending buf", result)
					stop = nil
					return
				}
			} else if err != nil {
				result.err = err
				glog.V(1).Infof("BlocksPipe @ %p got read error: %v", result, err)
				close(data)
				return
			} else {
				// 0 bytes read, but no error, so put that buffer back in the empty
				// channel.
				glog.V(1).Infof("BlocksPipe @ %p in.Read returned 0, nil", result)
				empty <- buf
			}
		}
	}()
	return result
}

// An io.ReaderCloser implementation, reading from a BlocksPipe.
type BlocksPipeReader struct {
	pipe* BlocksPipe
	mutex sync.Mutex
	front []byte
	offset int
}

// Reads up to min(len(b), N) bytes from the BlocksPipe, where N is the
// blockSize specified when the BlocksPipe was created (i.e. this implementation
// reads from only a single block in the BlocksPipe, in order to simplify the
// implementation).
func (p *BlocksPipeReader) Read(b []byte) (int, error) {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	if p.pipe == nil {
		glog.V(1).Infof("BlocksPipeReader @ %p already closed", p)
		return 0, io.EOF
	}
	if p.front != nil {
		n := copy(b, p.front[p.offset:])
//		glog.V(1).Infof("BlocksPipeReader @ %p copied %d bytes of %d in existing buffer to caller", p, n, len(p.front) - p.offset)
		p.offset += n
		if p.offset >= len(p.front) {
//			glog.V(1).Infof("BlocksPipeReader @ %p drained existing buffer", p)
			p.pipe.empty <- p.front
			p.front = nil
		}
		return n, nil
	}
	var ok bool
	p.front, ok = <- p.pipe.data
	if !ok {
		err := p.pipe.err
		glog.V(1).Infof("BlocksPipeReader @ %p data channel closed, error: %v", p, err)
		p.pipe = nil
		if err == nil {
			err = io.EOF
		}
		return 0, err
	}
	n := copy(b, p.front)
//	glog.V(1).Infof("BlocksPipeReader @ %p copied %d bytes of %d to caller", p, n, len(p.front))
	if n >= len(p.front) {
		p.pipe.empty <- p.front
		p.front = nil
	} else {
		p.offset = n
	}
	return n, nil
}
// Closes the underlying BlocksPipe, which in turn closes the input.
func (p *BlocksPipeReader) Close() error {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	if p.pipe == nil {
		return io.EOF
	}
	close(p.pipe.stop)
	p.pipe = nil
	return nil
}

func BlocksPipeToReadCloser(pipe *BlocksPipe) io.ReadCloser {
	return &BlocksPipeReader{pipe: pipe}
}

// Entry point for BlocksPipe and BlocksPipeReader: creates a BlocksPipe and
// exposes it as an io.ReadCloser (implemented by BlocksPipeReader).
func NewReadCloserPump(in io.Reader, blockSize, blockCount int) io.ReadCloser {
	pipe := ReadIntoBlocksPipe(in, blockSize, blockCount)
	return BlocksPipeToReadCloser(pipe)
}
