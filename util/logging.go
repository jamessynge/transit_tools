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

func IndentedInfof(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	prefix := levelPrefix[0 : enterExitDepth * kLevelPrefixUnit]
	glog.InfoDepth(1, prefix, " ", msg)
}

func IndentedVInfof(v glog.Level, format string, args ...interface{}) {
	if glog.V(v) {
		prefix := levelPrefix[0 : enterExitDepth * kLevelPrefixUnit]
		msg := fmt.Sprintf(format, args...)
		glog.InfoDepth(1, prefix, msg)
	}
}
