package main 

import (
"fmt"
"math"
"time"
"bytes"
)

func Test_mod() {
	for i := -360; i <= 360; i++ {
		fmt.Printf("mod(%d, 360) = %f\n", i, math.Mod(float64(i), 360))
	}
}

func Test_Time_Format() {
	// layout shows by example how the reference time should be represented.
	const layout = "Jan 2, 2006 at 3:04pm (MST) 2006-01-02_1504"
	t := time.Date(2009, time.November, 10, 15, 0, 0, 0, time.Local)
	fmt.Println(t.Format(layout))
	fmt.Println(t.UTC().Format(layout))
}

func TestQuotedPrinting() {
	s := "\t\002abc123"
	fmt.Printf(" v of s = %v\n", s)
	fmt.Printf("#v of s = %#v\n", s)
	fmt.Printf(" q of s = %q\n", s)
	fmt.Printf("#q of s = %#q\n", s)
	fmt.Printf("+q of s = %+q\n", s)
	fmt.Println()

	var b []byte = []byte(s)
	
	fmt.Printf(" v of b = %v\n", b)
	fmt.Printf("#v of b = %#v\n", b)
	fmt.Printf(" q of b = %q\n", b)
	fmt.Printf("#q of b = %#q\n", b)
	fmt.Printf("+q of b = %+q\n", b)

	var n []byte = nil

	fmt.Printf(" v of n = %v\n", n)
	fmt.Printf("#v of n = %#v\n", n)
	fmt.Printf(" q of n = %q\n", n)
	fmt.Printf("#q of n = %#q\n", n)
	fmt.Printf("+q of n = %+q\n", n)
}

const (
comment_start = "<!--"
comment_end = "-->"
pi_start = "<?"
pi_end = "?>"
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
		if offset + len(pat) <= length {
			for n := 0; n < len(pat); n++ {
				if pat[n] != body[n + offset] { return false }
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
						"Reached end of body before end of processing instruction " +
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
						"Reached end of body before end of comment that " +
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

func Use_FindRootXmlElementOffset(bodyStr string) {
	body := []byte(bodyStr)
	w, r, err := FindRootXmlElementOffset(body)
	fmt.Println(w, r, err)
	if err == nil {
		fmt.Printf("a: '%s'\n\n", body[0:w])
		fmt.Printf("b: '%s'\n\n", body[w:r])
		fmt.Printf("c: '%s'\n\n", body[r:])
	}
}
func Test_FindRootXmlElementOffset() {
	Use_FindRootXmlElementOffset(`b`)
	Use_FindRootXmlElementOffset(``)
	Use_FindRootXmlElementOffset(`<b`)
	Use_FindRootXmlElementOffset(`<!--`)
	Use_FindRootXmlElementOffset(`<??><abc>`)

	s := `<?xml version="1.0" encoding="utf-8" ?> 
<body copyright="All data copyright MBTA 2014.">
  <route tag="76" title="76" scheduleClass="20140830" serviceClass="Friday"
         direction="Inbound">
    <header>
      <stop tag="85231">Lincoln Lab</stop>
      <stop tag="86179_1">Civil Air Terminal</stop>
      <stop tag="141_ar">Alewife Station Busway</stop>
    </header>
    <tr blockID="T350_173">
      <stop tag="85231" epochTime="21600000">06:00:00</stop>
      <stop tag="86179_1" epochTime="-1">--</stop>
      <stop tag="141_ar" epochTime="23820000">06:37:00</stop>
    </tr>
  </route>
</body>`

	Use_FindRootXmlElementOffset(s)
}



func main() {
	Test_FindRootXmlElementOffset()
}
