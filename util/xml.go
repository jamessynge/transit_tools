package util

import (
	"bytes"
	"fmt"
)

const (
	kCommentStart  = "<!--"
	kCommentEnd    = "-->"
	kProcInstStart = "<?"
	kProcInstEnd   = "?>"
)

var commentStart, commentEnd, procInstStart, procInstEnd, tagEnd []byte

func init() {
	commentStart = []byte(kCommentStart)
	commentEnd = []byte(kCommentEnd)
	procInstStart = []byte(kProcInstStart)
	procInstEnd = []byte(kProcInstEnd)
	tagEnd = []byte{'>'}
}

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
		if lookingAt([]byte(kProcInstStart)) {
			rest := body[offset:]
			n := bytes.Index(rest, []byte(kProcInstEnd))
			if n < 0 {
				err = fmt.Errorf(
					"Reached end of body before end of processing instruction "+
						"that started at offset %d", offset)
				return
			}
			offset += (n + len(kProcInstEnd))
			whitespaceStart = offset
			continue
		}

		// Skip over comments.
		if lookingAt([]byte(kCommentStart)) {
			rest := body[offset:]
			n := bytes.Index(rest, []byte(kCommentEnd))
			if n < 0 {
				err = fmt.Errorf(
					"Reached end of body before end of comment that "+
						"started at offset %d", offset)
				return
			}
			offset += (n + len(kCommentEnd))
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

func FindCommentsInXml(body []byte) (startAndEnds []int, err error) {
	const kProcInstSearch = `"'?`
	const kTagSearch = `"'>`

	offset := 0
	for {
		// Find start of next tag.
		pos := bytes.IndexByte(body[offset:], '<')
		if pos == -1 {
			return
		}
		start := offset + pos
		if bytes.HasPrefix(body[start:], commentStart) {
			startAndEnds = append(startAndEnds, start)
			offset = start + len(kCommentStart)
			rest := body[offset:]
			pos = bytes.Index(rest, commentEnd)
			if pos == -1 {
				err = fmt.Errorf("Comment starting at %d is not terminated", start)
				return
			}
			offset += pos + len(kCommentEnd)
			startAndEnds = append(startAndEnds, offset)
			continue
		}
		var ending []byte
		var search string
		if bytes.HasPrefix(body[start:], procInstStart) {
			ending = procInstEnd
			search = kProcInstSearch
			offset = start + len(procInstStart)
		} else {
			ending = tagEnd
			search = kTagSearch
			offset = start + 1
		}
		for {
			// Search for strings (quoted attribute values) of which there
			// may be many, or the end of the tag.
			pos = bytes.IndexAny(body[offset:], search)
			if pos == -1 {
				err = fmt.Errorf("Tag starting at %d is not terminated", start)
				return
			}
			offset += pos
			if bytes.HasPrefix(body[offset:], ending) {
				// Found the end of the tag.
				offset += len(ending)
				break
			}
			// Quoted string. Find matching close quote.
			pos = bytes.IndexByte(body[offset+1:], body[offset])
			if pos == -1 {
				err = fmt.Errorf("Quoted string starting at %d is not terminated", offset)
				return
			}
			offset += 1 + pos
		}
	}
}
