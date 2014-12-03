package nblocations

// Oddities seen in the data:
//
//  	Apparently when a bus is changing from one route or one direction to
//		another, multiple reports may be issued with the same exact location
//    AND time, even though the change is happening well after the location
//    report was taken.  Sometimes multiple changes in routeTag or dirTag are
//		seen, including returning to  the original value.
//
//    Sometimes an old (previous) location report for a vehicle is returned
//    with the same old time, after a newer report has been returned.

import (
	"github.com/golang/glog"
	"math"
	"github.com/jamessynge/transit_tools/nextbus"
	"sort"
	"time"
	"github.com/jamessynge/transit_tools/util"
)

type VehicleAggregator interface {
	// Insert new reports
	Insert(locations []*nextbus.VehicleLocation)

	// Get latest report for the specified vehicle
	GetVehicle(id string) *nextbus.VehicleLocation

	// Get latest report for every vehicle.
	GetAllVehicles() []*nextbus.VehicleLocation

	// Remove reports where we know this isn't the latest report for the vehicle.
	RemoveStaleReports() []*nextbus.VehicleLocation

	// Close the aggregator (i.e. at shutdown), returning all of the "active"
	// and stale reports.
	Close() []*nextbus.VehicleLocation
}

type aggregatingVechicleLocation struct {
	firstReport         *nextbus.VehicleLocation
	lastReport          *nextbus.VehicleLocation
	numReports          int
	sumUnixMilliseconds int64
	unseenCount         int
}

func (p *aggregatingVechicleLocation) IsSame(
	report *nextbus.VehicleLocation) bool {
	return p.firstReport != nil && p.firstReport.IsSameReport(report)
}

// When a bus is stopped at the end of a trip, NextBus (or MBTA) may return a
// report showing no change in report time, yet the dirTag or routeTag may
// have changed.  We can aggregate such reports to compute the time of the
// location report.
func (p *aggregatingVechicleLocation) IsAggregatable(
	report *nextbus.VehicleLocation) bool {
	first := p.firstReport
	return (first != nil && first.IsAlmostSameTime(report) &&
		first.IsSameVehiclePosition(report))
}

// ProduceOutput returns a VehicleLocation with the average of all the times
// accumulated in sumUnixMilliseconds. Does not modify *p.
// Yes, this is ridiculous overkill: it produces sub-second precision on
// the time of a vehicle location report by averaging the times from
// several report messages.  But, kind of fun to create! ;-)
func (p *aggregatingVechicleLocation) ProduceOutput() (
	priorLocation *nextbus.VehicleLocation) {
	if p.numReports > 0 {
		if p.numReports > 1 {
			floatAvgMillis := float64(p.sumUnixMilliseconds) / float64(p.numReports)
			avgUnixMilliseconds := int64(math.Floor(floatAvgMillis + 0.5))
			t := time.Unix(avgUnixMilliseconds/1000,
				(avgUnixMilliseconds%1000)*1000000)

			glog.V(2).Infof(
				"Merged %d reports for vehicle %v to produce time: %v",
				p.numReports, p.firstReport.VehicleId, t)

			priorLocation = new(nextbus.VehicleLocation)
			*priorLocation = *p.firstReport
			priorLocation.Time = t
		} else {
			priorLocation = p.firstReport
		}
	}
	return
}

type vehicleAggregator struct {
	// Multiple reports averaged for the same vehicle.
	aggregatingVehicles map[string]*aggregatingVechicleLocation

	// Latest report for each vehicle.
	latestReports map[string]*nextbus.VehicleLocation

	// When the oldest report was issued; kept with the aim of being able to
	// readily determine when we can flush queued reports (i.e. in time order).
	oldestReport time.Time

	// Queue of locations to be sorted in time order,
	// and written to the OutputChan.
	queuedReports []*nextbus.VehicleLocation
}

func MakeVehicleAggregator() VehicleAggregator {
	return &vehicleAggregator{
		aggregatingVehicles: make(map[string]*aggregatingVechicleLocation),
		latestReports:       make(map[string]*nextbus.VehicleLocation),
	}
}

// We operate here on the assumption that unseen vehicles have either stopped
// reporting or we're no longer requesting reports as old as the vehicle's
// latest report.

func (va *vehicleAggregator) emitStale(p *aggregatingVechicleLocation) *nextbus.VehicleLocation {
	loc1 := p.ProduceOutput()
	if loc1 == nil {
		glog.Error("ProduceOutput returned nil!")
		return nil
	}
	va.queuedReports = append(va.queuedReports, loc1)
	glog.V(2).Infof("emitStale for vehicle %v @ %s", loc1.VehicleId, loc1.Time)
	if p.lastReport == nil {
		return loc1
	}

	p.firstReport, p.lastReport = p.lastReport, nil
	loc2 := p.ProduceOutput()
	if loc2 == nil {
		glog.Error("ProduceOutput returned nil, definitely expected a VL!")
		return loc1
	}
	ms := util.TimeToUnixMillis(loc1.Time)
	loc2.Time = util.UnixMillisToTime(ms + 1)
	va.queuedReports = append(va.queuedReports, loc2)
	glog.V(1).Infof(`second emitStale for vehicle %v
 First Stale: %v
Second Stale: %v`,
		loc1.VehicleId, loc1.ToCSVFields(), loc2.ToCSVFields())
	return loc2
}

func (va *vehicleAggregator) insertOneReport(loc *nextbus.VehicleLocation) {
	id := loc.VehicleId
	p, ok := va.aggregatingVehicles[id]
	if !ok {
		// Haven't seen this vehicle recently.
		va.aggregatingVehicles[id] = &aggregatingVechicleLocation{
			firstReport:         loc,
			numReports:          1,
			sumUnixMilliseconds: loc.UnixMilliseconds(),
		}
		va.latestReports[id] = loc
		glog.V(2).Infof("First report for vehicle %v", id)
		return
	}
	p.unseenCount = 0
	if p.numReports > 0 {
		if p.IsAggregatable(loc) {
			p.numReports++
			p.sumUnixMilliseconds += loc.UnixMilliseconds()
			glog.V(2).Infof("Merging report #%d for vehicle %v", p.numReports, id)
			if p.firstReport.RouteTag != loc.RouteTag ||
				p.firstReport.DirTag != loc.DirTag {

				last := p.lastReport
				if last != nil && last.RouteTag != loc.RouteTag && last.DirTag != loc.DirTag {
					// Changed routes twice!
					glog.Warningf(`Aggregating VERY changed route reports for %s
First Report: %v
 Last Report: %v
  New Report: %v`,
						id, p.firstReport.ToCSVFields(),
						last.ToCSVFields(), loc.ToCSVFields())
				}
				p.lastReport = loc
			} else if p.lastReport != nil {
				glog.Errorf(`Expected lastReport to be nil for  %s
First Report: %v
 Last Report: %v
  New Report: %v`,
					id, p.firstReport.ToCSVFields(),
					p.lastReport.ToCSVFields(), loc.ToCSVFields())
				p.lastReport = nil
			}
			return
		}
		diff := p.firstReport.Time.Sub(loc.Time)
		if p.firstReport.Time.After(loc.Time) {
			if p.firstReport.IsSameReportExceptTime(loc) {
				glog.Warningf(`Reports for %s vary only by time:
  Old time: %s
  New time: %s
Difference: %s`, id, p.firstReport.Time, loc.Time, diff)
			} else {
				// Considering this to be an error because the change in other
				// attributes implies that dropping is a loss of information.
				// I've seen this occur though when NextBus has changed the
				// dirTag (e.g. from Inbound to Outbound without changing anything
				// else, including not changing the time of the last report); then the
				// next report for the vehicle is at a new time, with the new dirTag.
				glog.Errorf(`New (changed) report for %s is from an earlier time by %s
  Old Report: %v
  New Report: %v`,
					id, diff, p.firstReport.ToCSVFields(), loc.ToCSVFields())
			}
			// Drop this, else it will cause the csv archive to be out of order.
			return
		}
		if glog.V(1) {
			if p.firstReport.IsSameReportExceptTime(loc) {
				glog.V(2).Infof(`Reports for %s vary only by time:
  Old time: %s
  New time: %s
Difference: %s`, id, p.firstReport.Time, loc.Time, diff)
			} else {
				glog.V(2).Infof(
					"Reports for %s are different:\n  Old Report: %v\n  New report: %v",
					id, p.firstReport.ToCSVFields(), loc.ToCSVFields())
			}
		}
		va.emitStale(p)
	} else {
		glog.Errorf("p.numReports = %v", p.numReports) // Shouldn't (currently) get here.
	}
	p.firstReport = loc
	p.lastReport = nil
	p.numReports = 1
	p.sumUnixMilliseconds = loc.UnixMilliseconds()
	va.latestReports[id] = loc
	return
}

// Insert new reports
func (va *vehicleAggregator) Insert(locations []*nextbus.VehicleLocation) {
	if len(locations) == 0 {
		// Happens if no vehicles have reported moving since the last time reports
		// were requested (t parameter in URL), or when the servers are doing their
		// daily reboot (~3am for NextBus/MBTA), or when we had some other error
		// fetching data.
		// Since I request reports with lots of overlap, the former is less likely.
		return
	}

	// We have some vehicle location reports.  Sort by id so that logs are
	// more useful for debugging.
	locations = append(([]*nextbus.VehicleLocation)(nil), locations...)
	nextbus.SortVehicleLocationsById(locations)
	seen := make(map[string]bool)
	oldSize := len(va.aggregatingVehicles)
	for _, v := range locations {
		if v.Time.Before(va.oldestReport) {
			glog.Warningf(`Report is before previous oldest report time
New Report: %v
  Old Time: %d (%s)`,
				v.ToCSVFields(), util.TimeToUnixMillis(va.oldestReport),
				va.oldestReport)
		}
		id := v.VehicleId
		seen[id] = true
		va.insertOneReport(v)
	}

	newCount := len(va.aggregatingVehicles) - oldSize
	oldCount := len(locations) - newCount
	unseenCount := oldSize - oldCount
	glog.V(1).Infof("Inserted %d new and %d old vehicles; %d were not seen",
		newCount, oldCount, unseenCount)

	// Find the oldest recently seen entry.
	recentlySeenThreshold := 3 // Make this a field in *va
	first := true
	unseenIds := make([]string, 0, unseenCount)
	var oldest time.Time
	for id, p := range va.aggregatingVehicles {
		if _, ok := seen[id]; !ok {
			// Didn't see this vehicle in the latest set of reports.
			unseenIds = append(unseenIds, id)
			p.unseenCount++
			if p.unseenCount > recentlySeenThreshold {
				continue
			}
		}
		t := p.firstReport.Time
		if first {
			first = false
			oldest = t
		} else {
			if t.Before(oldest) {
				oldest = t
			}
		}
	}
	glog.V(1).Infof("Oldest recently seen report @ %s", oldest)
	// Move oldest back by 3 seconds to allow for an excessive amount of movement
	// of the average (i.e. I've seen it move more than I previously allowed for).
	oldest = oldest.Add(time.Duration(-3) * time.Second)
	va.oldestReport = oldest

	// Enqueue the unseen entries older than oldest recently seen entry.
	queuedCount := len(va.queuedReports)
	sort.Strings(unseenIds)
	for _, id := range unseenIds {
		p, ok := va.aggregatingVehicles[id]
		if !ok {
			glog.Errorf("Vehicle %q is missing!", id)
			continue
		}
		if p.unseenCount == 0 {
			glog.Errorf("Unseen vehicle %q has unseenCount of 0!", id)
			continue
		}
		d := oldest.Sub(p.firstReport.Time)
		// Allow for a fair amount of movement of the average
		if d.Seconds() > 3.0 {
			loc := va.emitStale(p)
			// Replace the latestReport which is based on the first report of the
			// aggregated group, with the generated report that has the average
			// time value.
			va.latestReports[id] = loc
			// Could just zero out various fields (i.e. reduce work the GC has to
			// do in reclaiming and then reallocating memory for this vehicle id).
			delete(va.aggregatingVehicles, id)
		}
	}
	glog.V(1).Infof("Queued %d stale entries", len(va.queuedReports)-queuedCount)
}

// Get latest report for the specified vehicle
func (va *vehicleAggregator) GetVehicle(id string) *nextbus.VehicleLocation {
	return va.latestReports[id]
}

// Get latest report for every vehicle.
func (va *vehicleAggregator) GetAllVehicles() []*nextbus.VehicleLocation {
	result := make([]*nextbus.VehicleLocation, 0, len(va.latestReports))
	for _, p := range va.latestReports {
		result = append(result, p)
	}
	return result
}

// Remove reports where we know this isn't the latest report for the vehicle.
func (va *vehicleAggregator) RemoveStaleReports() []*nextbus.VehicleLocation {
	nextbus.SortVehicleLocationsByDateAndId(va.queuedReports)
	ndx := sort.Search(len(va.queuedReports), func(i int) bool {
		return !va.oldestReport.After(va.queuedReports[i].Time)
	})
	result := make([]*nextbus.VehicleLocation, ndx)
	if ndx > 0 {
		copy(result, va.queuedReports[0:ndx])
		// Might be more efficient if I sorted in reverse order so that there is
		// no need to copy the kept elements.
		newLen := len(va.queuedReports) - ndx
		copy(va.queuedReports, va.queuedReports[ndx:])
		va.queuedReports = va.queuedReports[0:newLen]
	}
	glog.V(1).Infof("RemoveStaleReports found %d reports older than %s",
		len(result), va.oldestReport)
	return result
}

func (va *vehicleAggregator) Close() []*nextbus.VehicleLocation {
	glog.Infof(
		"Close() found %d vehicles being aggregated, and %d queued reports",
		len(va.aggregatingVehicles), len(va.queuedReports))
	for _, p := range va.aggregatingVehicles {
		if p.numReports > 0 && p.firstReport != nil {
			va.emitStale(p)
		}
	}
	nextbus.SortVehicleLocationsByDateAndId(va.queuedReports)
	result := va.queuedReports
	va.aggregatingVehicles = nil
	va.latestReports = nil
	va.queuedReports = nil
	return result
}
