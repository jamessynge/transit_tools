package util

import (
	"bytes"
	"fmt"
)

const (
	comment_start = "<!--"
	comment_end   = "-->"
	pi_start      = "<?"
	pi_end        = "?>"
)

func FindRootXmlElementOffset(body []byte) (
	whitespaceStart, rootOffset int, err error) {
	defer func() {
		if err != nil {
			whitespaceStart = -1
			rootOffset = -1
		}
	}()

	whitespaceStart = 0
	rootOffset = 0
	length := len(body)
	offset := 0

	isWhitespace := func(b byte) bool {
		switch b {
		case ' ', '\t', '\n', '\r':
			return true
		default:
			return false
		}
	}

	lookingAt := func(pat []byte) bool {
		if offset+len(pat) <= length {
			for n := 0; n < len(pat); n++ {
				if pat[n] != body[n+offset] {
					return false
				}
			}
			return true
		}
		return false
	}

	for offset < length {
		b := body[offset]

		// Skip over whitespace
		if isWhitespace(b) {
			offset++
			continue
		}

		// Should be the start of a tag
		if b != '<' {
			err = fmt.Errorf("Unexpected character (%c, %U) at offset %d", b, b, offset)
			return
		}

		// Skip over processing instructions (and XML declaration).
		if lookingAt([]byte(pi_start)) {
			rest := body[offset:]
			n := bytes.Index(rest, []byte(pi_end))
			if n < 0 {
				err = fmt.Errorf(
					"Reached end of body before end of processing instruction "+
						"that started at offset %d", offset)
				return
			}
			offset += (n + len(pi_end))
			whitespaceStart = offset
			continue
		}

		// Skip over comments.
		if lookingAt([]byte(comment_start)) {
			rest := body[offset:]
			n := bytes.Index(rest, []byte(comment_end))
			if n < 0 {
				err = fmt.Errorf(
					"Reached end of body before end of comment that "+
						"started at offset %d", offset)
				return
			}
			offset += (n + len(comment_end))
			whitespaceStart = offset
			continue
		}

		rootOffset = offset
		return
	}
	err = fmt.Errorf(
		"Reached end of body at offset %d before finding root element.", offset)
	return
}
