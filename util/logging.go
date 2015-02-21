package util

import (
	"fmt"
	"strings"

	"github.com/golang/glog"
)

const (
	kEnterPrefix = "> "
	kExitPrefix = "< "
	kLevelPrefix = " |"
)

const (
	kEnterPrefixUnit = len(kEnterPrefix)
	kExitPrefixUnit = len(kExitPrefix)
	kLevelPrefixUnit = len(kLevelPrefix)
)

var (
	enterExitDepth int = 0
	enterExitPrefixMaxDepth = 1
	enterPrefix = kEnterPrefix
	exitPrefix = kExitPrefix
	levelPrefix = kLevelPrefix
)

func updateLoggingPrefixes() {
	if enterExitDepth <= 0 {
		glog.Fatalf("enterExitDepth (%d) is < 0!!!", enterExitDepth)
	}
	if enterExitDepth > enterExitPrefixMaxDepth {
		enterPrefix = strings.Repeat(enterPrefix, 2)
		exitPrefix = strings.Repeat(exitPrefix, 2)
		levelPrefix = strings.Repeat(levelPrefix, 2)
		enterExitPrefixMaxDepth *= 2

//		fmt.Println("enterPrefix:", enterPrefix)
//		fmt.Println("exitPrefix:", exitPrefix)
//		fmt.Println("levelPrefix:", levelPrefix)
	}
}

func getLoggingPrefix(pStr *string) string {
	return enterPrefix[0 : 2*enterExitDepth]
}

// Logs a prefix and a formatted message when called, and returns another
// method that glog's another prefix and the same method when the returned
// method is called. This is in support of logging the entry into and exit
// from a method, such as:
//
//    func Foo(arg type) {
//      defer util.EnterExitInfof("Foo(%v)", arg)()
//      // Some work...
//    }
//
// Note the final pair of parenthesis on the defer, which triggers the calling
// of the returned function when the function Foo exits.
// glog.InfoDepth is used so that the logged location is the line containing
// the caller's defer statement.
// The prefix indicates the depth of nesting of functions that call
// EnterExitInfof and EnterExitVInfof, and the prefix is different
// for entry and exit, so that is isn't too difficult
// to tell the difference in the logged output.
func EnterExitInfof(format string, args ...interface{}) func() {
	enterExitDepth++
	updateLoggingPrefixes()
	p1 := enterPrefix[0 : enterExitDepth * kEnterPrefixUnit]
	p2 := exitPrefix[0 : enterExitDepth * kExitPrefixUnit]
	msg := fmt.Sprintf(format, args...)
	glog.InfoDepth(1, p1, msg)
	return func() {
//		fmt.Printf("\nstarting EnterExitInfof deferred function at depth %d\n\n", enterExitDepth)
		glog.InfoDepth(2, p2, msg)
		enterExitDepth--
//		fmt.Printf("\n  ending EnterExitInfof deferred function at depth %d\n\n", enterExitDepth)
	}
}

// Like EnterExitInfof, but takes a glog.Level specifying the verbosity
// level of the log message.
func EnterExitVInfof(v glog.Level, format string, args ...interface{}) func() {
	if !glog.V(v) {
		return func() {}
	}
	enterExitDepth++
	updateLoggingPrefixes()
	p1 := enterPrefix[0 : enterExitDepth * kEnterPrefixUnit]
	p2 := exitPrefix[0 : enterExitDepth * kExitPrefixUnit]
	msg := fmt.Sprintf(format, args...)
	glog.InfoDepth(1, p1, msg)
	return func() {
//		fmt.Printf("\nin EnterExitVInfof deferred function at depth %d\n\n", enterExitDepth)
		glog.InfoDepth(2, p2, msg)
		enterExitDepth--
	}
}

// Logs a prefix and formatted message, the prefix being determined by the
// nesting depth of calls to EnterExitInfof and EnterExitVInfof.
// glog.InfoDepth is used so that the logged location is the line containing
// the caller's defer statement.
func IndentedInfof(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	prefix := levelPrefix[0 : enterExitDepth * kLevelPrefixUnit]
	glog.InfoDepth(1, prefix, " ", msg)
}

// Like IndentedInfof, but takes a glog.Level specifying the verbosity
// level of the log message.
func IndentedVInfof(v glog.Level, format string, args ...interface{}) {
	if glog.V(v) {
		prefix := levelPrefix[0 : enterExitDepth * kLevelPrefixUnit]
		msg := fmt.Sprintf(format, args...)
		glog.InfoDepth(1, prefix, msg)
	}
}
