package util

import (
	"flag"
	"fmt"
	"testing"
	"time"
	
	"reflect"

	"github.com/jamessynge/transit_tools/compare"
)

var utc, samoa, chicago, usEastern, gmtMinus12 *time.Location

func LoadLocation(name string, t *testing.T) *time.Location {
	loc, err := time.LoadLocation(name)
	if err != nil {
		t.Fatalf("Unable to load %s: %s", name, err)
	}
	return loc
}

type testflags struct {
	fs *flag.FlagSet

	// Flag vars with defaults
	localDef *time.Location
	utcDef *time.Location
	chicagoDef *time.Location

	// Flag vars without defaults
	tl1 *time.Location
	tl2 *time.Location
}

func NewTestFlags(t *testing.T) *testflags {
	if utc == nil {
		fmt.Println("Initializing time.Locations")
		utc = LoadLocation("UTC", t)
		chicago = LoadLocation("America/Chicago", t)
		usEastern = LoadLocation("US/Eastern", t)
		samoa = LoadLocation("US/Samoa", t)
		gmtMinus12 = LoadLocation("Etc/GMT-12", t)  // Zero population TZ
	}
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	tf := &testflags{
		fs: fs,
		localDef: time.Local,
		utcDef: time.UTC,
		chicagoDef: chicago,
	}
	NewTimeLocationVarFlagSet(
			fs, &tf.localDef, "LocalDefault", "local default usage")
	NewTimeLocationVarFlagSet(
			fs, &tf.utcDef, "UTCDefault", "utc default usage")
	NewTimeLocationVarFlagSet(
			fs, &tf.chicagoDef, "ChicagoDefault", "chicago default usage")
	NewTimeLocationVarFlagSet(
			fs, &tf.tl1, "tl1", "tl2 usage")
	NewTimeLocationVarFlagSet(
			fs, &tf.tl2, "tl2", "tl2 usage")
	return tf
}

func (p *testflags) Parse(args ... string) error {
	return p.fs.Parse(args)
}

func ExpectEQTimeLocations(a, b *time.Location, t *testing.T) {
	config := new(compare.Config)
//	config.Logger = t
	eq, diffs := compare.DeepCompare3(a, b, config)
	if !eq || len(diffs) > 0 {
		t.Errorf(" diffs: %s", diffs)
	}
}

func TestTimeLocationVarFlagDefaults(t *testing.T) {
	tf := NewTestFlags(t)
	ExpectEQTimeLocations(time.Local, tf.localDef, t)
	ExpectEQTimeLocations(utc, tf.utcDef, t)
	ExpectEQTimeLocations(chicago, tf.chicagoDef, t)
	ExpectEQTimeLocations(nil, tf.tl1, t)
	ExpectEQTimeLocations(nil, tf.tl2, t)

	err := tf.Parse()
	if err != nil {
		t.Fatalf("Unable to parse empty command line: %s", err)
	}
	ExpectEQTimeLocations(time.Local, tf.localDef, t)
	ExpectEQTimeLocations(utc, tf.utcDef, t)
	ExpectEQTimeLocations(chicago, tf.chicagoDef, t)
	ExpectEQTimeLocations(nil, tf.tl1, t)
	ExpectEQTimeLocations(nil, tf.tl2, t)
}

func TestTimeLocationVarFlagOverrideDefault(t *testing.T) {
	tf := NewTestFlags(t)

	err := tf.Parse("--LocalDefault=Etc/GMT-12",
	                "--UTCDefault=America/Chicago",
	                "--ChicagoDefault=US/Eastern",
	                "--tl1=UTC",
	                "--tl2", "Local")
	if err != nil {
		t.Fatalf("Unable to parse empty command line: %s", err)
	}
	ExpectEQTimeLocations(gmtMinus12, tf.localDef, t)
	ExpectEQTimeLocations(chicago, tf.utcDef, t)
	ExpectEQTimeLocations(usEastern, tf.chicagoDef, t)
	ExpectEQTimeLocations(time.UTC, tf.tl1, t)
	ExpectEQTimeLocations(time.Local, tf.tl2, t)
}

// TODO Restore/repair tests for NewTimeLocationFlag and
// NewTimeLocationFlagSet (which return *util.TimeLocation values),
// in addition to the above tests where the caller passes in a **time.Location
// value, which must be done in a function (e.g. func init() {...}).

/*
func TestTimeLocationFlagDefaults(t *testing.T) {
	tf := NewTestFlags(t)
	ExpectEQTimeLocations(time.Local, tf.localDef, t)
	ExpectEQTimeLocations(&utc, tf.utcDef, t)
	ExpectEQTimeLocations(&chicago, tf.chicagoDef, t)
	ExpectEQTimeLocations(nil, tf.tl1, t)
	ExpectEQTimeLocations(nil, tf.tl2, t)

	err := tf.Parse()
	if err != nil {
		t.Fatalf("Unable to parse empty command line: %s", err)
	}
	ExpectEQTimeLocations(time.Local, tf.localDef, t)
	ExpectEQTimeLocations(&utc, tf.utcDef, t)
	ExpectEQTimeLocations(&chicago, tf.chicagoDef, t)
	ExpectEQTimeLocations(nil, tf.tl1, t)
	ExpectEQTimeLocations(nil, tf.tl2, t)
}

func TestTimeLocationFlagOverrideDefault(t *testing.T) {
	tf := NewTestFlags(t)

	err := tf.Parse("--LocalDefault=Etc/GMT-12",
	                "--UTCDefault=America/Chicago",
	                "--ChicagoDefault=US/Eastern",
	                "--tl1=UTC",
	                "--tl2", "Local")
	if err != nil {
		t.Fatalf("Unable to parse empty command line: %s", err)
	}
	ExpectEQTimeLocations(&gmtMinus12, tf.localDef, t)
	ExpectEQTimeLocations(&chicago, tf.utcDef, t)
	ExpectEQTimeLocations(&usEastern, tf.chicagoDef, t)
	ExpectEQTimeLocations(time.UTC, tf.tl1, t)
	ExpectEQTimeLocations(time.Local, tf.tl2, t)
}
*/
func TestTimeLocationApplied(t *testing.T) {
	makeTime := func(loc *time.Location) time.Time {
		return time.Date(2014, 12, 1, 12, 30, 0, 0, loc)
	}
	t1a := makeTime(time.UTC)
	t1b := makeTime(utc)
	compare.ExpectEqual(t.Log, &t1a, &t1b)
	t2 := makeTime(LoadLocation("Etc/GMT-1", t))
	delta := t1a.Sub(t2)
	fmt.Printf("delta: %s\n", delta)
	



	
	t.Fail()
}
