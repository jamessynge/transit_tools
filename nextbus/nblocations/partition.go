package nblocations

import (
	"encoding/xml"
	"fmt"
	"io"

	"github.com/golang/glog"

	"github.com/jamessynge/transit_tools/geo"
)

////////////////////////////////////////////////////////////////////////////////

func CreateSouthNorthPartitioners(
	loLon, hiLon geo.Longitude,
	loLat, hiLat geo.Latitude,
	level, levelLimit uint, cutsPerLevel int,
	sortedLocations []geo.Location,
	sortedWestToEast bool) decoderPartitioner {
	if level >= levelLimit {
		return CreateLeafPartitioner(loLon, hiLon, loLat, hiLat, level)
	}

	snLocations := sortedLocations
	sortedLocations = nil
	if sortedWestToEast {
		geo.SortSouthToNorth(snLocations)
	}

	glog.Infof("%s CreateSouthNorthPartitioners(%v, %v, %v, %v, %d, len=%d)",
		"                      "[:level],
		loLon, hiLon, loLat, hiLat, level, len(snLocations))
	if len(snLocations) < int(cutsPerLevel)*10 {
		glog.Fatalf("Too few locations (%d) in S-N partitions at level %d",
			len(snLocations), level)
	}
	p := &SouthNorthPartitioners{
		PartitionsBase: PartitionsBase{
			RegionBase: RegionBase{
				West:  loLon,
				East:  hiLon,
				South: loLat,
				North: hiLat,
			},
			minCoord: float64(loLat),
			maxCoord: float64(hiLat),
			level:    level,
		},
	}

	if level < 2 {
		// Special handling of the first two levels, where we always do 2 cuts
		// producing 3 partitions at each level; the outer two partitions at
		// these levels will be leaves, yet will basically just contain a few
		// points that are outliers. The single other non-leaf will cover
		// essentially all of the transit region.
		if loLat != -90 || hiLat != 90 {
			glog.Fatalf("Latitude expected to be full 180, not %v to %v",
				loLat, hiLat)
		}

		southern, northern := getLocationsNearExtrema(snLocations)
		locationsLoLat := southern.Lat
		locationsHiLat := northern.Lat
		glog.Infof("special latitude range: %v to %v", locationsLoLat, locationsHiLat)

		// Remove the locations that are outside that latitude range.
		for n := range snLocations {
			if locationsLoLat > snLocations[n].Lat {
				continue
			}
			if n > 0 {
				snLocations = snLocations[n:]
			}
			break
		}
		for n := len(snLocations); true; {
			n--
			if locationsHiLat < snLocations[n].Lat {
				continue
			}
			snLocations = snLocations[0 : n+1]
			break
		}

		p.SubPartitions = append(p.SubPartitions,
			CreateLeafPartitioner(loLon, hiLon, loLat, locationsLoLat, level+1))

		p.cutPoints = append(p.cutPoints, float64(locationsLoLat))

		if level == 0 {
			p.SubPartitions = append(p.SubPartitions,
				CreateWestEastPartitioners(
					loLon, hiLon, locationsLoLat, locationsHiLat,
					level+1, levelLimit, cutsPerLevel, snLocations, false))
		} else {
			p.SubPartitions = append(p.SubPartitions,
				createPartitioner(
					loLon, hiLon, locationsLoLat, locationsHiLat,
					level+1, levelLimit, cutsPerLevel, snLocations, false))
		}

		p.cutPoints = append(p.cutPoints, float64(locationsHiLat))

		p.SubPartitions = append(p.SubPartitions,
			CreateLeafPartitioner(loLon, hiLon, locationsHiLat, hiLat, level+1))
		return p
	}

	// Normal case (i.e. not including the poles or international dateline).
	// The loop produces the first cutsPerLevel partitions, and the code after
	// the loop produces the last partition.

	minCoord := loLat
	for c := cutsPerLevel + 1; c > 1; c-- {
		// Take the first n locations (this partition) out of the s-n sorted
		// locations, and use them to create a sub-partitioner.
		n := len(snLocations) / int(c)
		partitionLocations := snLocations[0:n]

		// The remaining locations are for the remaining partition(s);
		// the first remaining location defines the Latitude limit (beyond)
		// for this partition.
		snLocations = snLocations[n:]
		maxCoord := snLocations[0].Lat

		// Create a sub-partition based on weRecords.
		sp := createPartitioner(
			loLon, hiLon, minCoord, maxCoord,
			level+1, levelLimit, cutsPerLevel,
			partitionLocations, false)

		p.cutPoints = append(p.cutPoints, float64(maxCoord))
		p.SubPartitions = append(p.SubPartitions, sp)

		minCoord = maxCoord
	}

	// Remaining locations are for the final partition.
	sp := createPartitioner(
		loLon, hiLon, minCoord, hiLat,
		level+1, levelLimit, cutsPerLevel,
		snLocations, false)
	p.SubPartitions = append(p.SubPartitions, sp)

	return p
}

////////////////////////////////////////////////////////////////////////////////

func CreateWestEastPartitioners(
	loLon, hiLon geo.Longitude,
	loLat, hiLat geo.Latitude,
	level, levelLimit uint, cutsPerLevel int,
	sortedLocations []geo.Location,
	sortedWestToEast bool) decoderPartitioner {
	if level >= levelLimit {
		return CreateLeafPartitioner(loLon, hiLon, loLat, hiLat, level)
	}

	weLocations := sortedLocations
	sortedLocations = nil
	if !sortedWestToEast {
		geo.SortWestToEast(weLocations)
	}

	glog.Infof("%s CreateWestEastPartitioners(%v, %v, %v, %v, %d, len=%d)",
		"                      "[:level],
		loLon, hiLon, loLat, hiLat, level, len(weLocations))

	if len(weLocations) < cutsPerLevel*10 {
		glog.Fatalf("Too few locations (%d) in W-E partitions at level %d",
			len(weLocations), level)
	}
	p := &WestEastPartitioners{
		PartitionsBase: PartitionsBase{
			RegionBase: RegionBase{
				West:  loLon,
				East:  hiLon,
				South: loLat,
				North: hiLat,
			},
			level:    level,
			minCoord: float64(loLon),
			maxCoord: float64(hiLon),
		},
	}
	if level < 2 {
		// Special handling of the first two levels, where we always do 2 cuts
		// producing 3 partitions at each level; the outer two partitions at
		// these levels will be leaves, yet will basically just contain a few
		// points that are outliers. The single other non-leaf will cover
		// essentially all of the transit region.
		if loLon != -180 || hiLon != 180 {
			glog.Fatalf("Longitude expected to be full 360, not %v to %v",
				loLon, hiLon)
		}

		western, eastern := getLocationsNearExtrema(weLocations)
		locationsLoLon := western.Lon
		locationsHiLon := eastern.Lon
		glog.Infof("special longitude range: %v to %v", locationsLoLon, locationsHiLon)

		// Remove the locations that are outside that longitude range.
		for n := range weLocations {
			if locationsLoLon > weLocations[n].Lon {
				continue
			}
			if n > 0 {
				weLocations = weLocations[n:]
			}
			break
		}
		for n := len(weLocations); true; {
			n--
			if locationsHiLon < weLocations[n].Lon {
				continue
			}
			weLocations = weLocations[0 : n+1]
			break
		}

		p.SubPartitions = append(p.SubPartitions,
			CreateLeafPartitioner(loLon, locationsLoLon, loLat, hiLat, level+1))

		p.cutPoints = append(p.cutPoints, float64(locationsLoLon))

		if level == 0 {
			p.SubPartitions = append(p.SubPartitions,
				CreateSouthNorthPartitioners(
					locationsLoLon, locationsHiLon, loLat, hiLat,
					level+1, levelLimit, cutsPerLevel,
					weLocations, true))
		} else {
			p.SubPartitions = append(p.SubPartitions,
				createPartitioner(
					locationsLoLon, locationsHiLon, loLat, hiLat,
					level+1, levelLimit, cutsPerLevel,
					weLocations, true))
		}

		p.cutPoints = append(p.cutPoints, float64(locationsHiLon))

		p.SubPartitions = append(p.SubPartitions,
			CreateLeafPartitioner(locationsHiLon, hiLon, loLat, hiLat, level+1))
		return p
	}

	// Normal case (i.e. not including the poles or international dateline).

	minCoord := loLon
	for c := cutsPerLevel + 1; c > 1; c-- {
		// Take the first n locations (this partition) out of the w-e sorted
		// locations, and use them to create a sub-partitioner.
		n := len(weLocations) / int(c)
		partitionLocations := weLocations[0:n]

		// The remaining w-e locations are for the remaining partitions;
		// the first remaining location defines the Longitude limit (beyond)
		// for this partition.
		weLocations = weLocations[n:]
		maxCoord := weLocations[0].Lon

		// Create a sub-partition based on partitionLocations.
		sp := createPartitioner(
			minCoord, maxCoord, loLat, hiLat,
			level+1, levelLimit, cutsPerLevel,
			partitionLocations, true)

		p.cutPoints = append(p.cutPoints, float64(maxCoord))
		p.SubPartitions = append(p.SubPartitions, sp)

		minCoord = maxCoord
	}

	// Remaining locations are for the final partition.
	sp := createPartitioner(
		minCoord, hiLon, loLat, hiLat,
		level+1, levelLimit, cutsPerLevel,
		weLocations, true)
	p.SubPartitions = append(p.SubPartitions, sp)

	return p
}

////////////////////////////////////////////////////////////////////////////////

func CreateLeafPartitioner(
	loLon, hiLon geo.Longitude, loLat, hiLat geo.Latitude,
	level uint) *LeafPartitioner {
	snDistance, weDistance := geo.MeasureCentralAxes(loLon, hiLon, loLat, hiLat)
	glog.V(1).Infof(
		"%s CreateLeafPartitioner(%v, %v, %v, %v, %d) S2N: %dm, W2E: %dm",
		"                      "[:level],
		loLon, hiLon, loLat, hiLat, level, int(snDistance), int(weDistance))

	fn := fmt.Sprintf("%s_%s-to-%s_%s.csv.gz",
		loLat.FormatDMS(), loLon.FormatDMS(),
		hiLat.FormatDMS(), hiLon.FormatDMS())
	glog.V(1).Infof("           will create %s", fn)

	//		// Add a header identifying the fields to the csv file.
	//		fieldNames := nextbus.VehicleCSVFieldNames()
	//		fieldNames[0] = "# " + fieldNames[0]
	//		p.csv.Write(fieldNames)

	return &LeafPartitioner{
		RegionBase: RegionBase{
			West:  loLon,
			East:  hiLon,
			South: loLat,
			North: hiLat,
		},
		FileName: fn,
		level:    level,
	}
}

////////////////////////////////////////////////////////////////////////////////
/*
func CreateGridPartitioner(
	loLon, hiLon geo.Longitude, loLat, hiLat geo.Latitude,
	level, levelLimit uint, snCuts, weCuts int,
	sortedLocations []geo.Location,
	sortedWestToEast bool, ) *GridPartitioner {
	snDistance, weDistance := geo.MeasureCentralAxes(loLon, hiLon, loLat, hiLat)
	glog.Infof(
		"%s CreateGridPartitioner(%v, %v, %v, %v, %d) S2N: %dm, W2E: %dm",
		"                      "[:level],
		loLon, hiLon, loLat, hiLat, level, int(snDistance), int(weDistance))

	




	fn := fmt.Sprintf("%s_%s-to-%s_%s.csv.gz",
		loLat.FormatDMS(), loLon.FormatDMS(),
		hiLat.FormatDMS(), hiLon.FormatDMS())
	glog.Infof("           will create %s", fn)

	//		// Add a header identifying the fields to the csv file.
	//		fieldNames := nextbus.VehicleCSVFieldNames()
	//		fieldNames[0] = "# " + fieldNames[0]
	//		p.csv.Write(fieldNames)

	return &LeafPartitioner{
		RegionBase: RegionBase{
			West:  loLon,
			East:  hiLon,
			South: loLat,
			North: hiLat,
		},
		FileName: fn,
		level:    level,
	}
}
*/
////////////////////////////////////////////////////////////////////////////////

// Choose a direction for partitioning based on the axis with the
// greatest distance (i.e. try to avoid making long thin rectangles).
func createPartitioner(
	loLon, hiLon geo.Longitude,
	loLat, hiLat geo.Latitude,
	level, levelLimit uint, cutsPerLevel int,
	sortedLocations []geo.Location,
	sortedWestToEast bool) decoderPartitioner {
	snDistance, weDistance := geo.MeasureCentralAxes(loLon, hiLon, loLat, hiLat)
	glog.Infof(
		"%s createPartitioner(%v, %v, %v, %v, %d) S2N: %dm, W2E: %dm",
		"                      "[:level],
		loLon, hiLon, loLat, hiLat, level, int(snDistance), int(weDistance))

	if weDistance >= snDistance {
		if weDistance > 500 {
			return CreateWestEastPartitioners(
				loLon, hiLon, loLat, hiLat,
				level, levelLimit, int(cutsPerLevel),
				sortedLocations, sortedWestToEast)
		}
	} else {
		if snDistance > 500 {
			return CreateSouthNorthPartitioners(
				loLon, hiLon, loLat, hiLat,
				level, levelLimit, int(cutsPerLevel),
				sortedLocations, sortedWestToEast)
		}
	}

	return CreateLeafPartitioner(loLon, hiLon, loLat, hiLat, level)
}

////////////////////////////////////////////////////////////////////////////////

// Create partitioners such that the resulting partition files are of roughly
// equal size (number of records).
func CreateAgencyPartitioner(agency string, samples []geo.Location,
	levelLimit uint, cutsPerLevel uint) *AgencyPartitioner {
	if levelLimit < 2 {
		glog.Fatalf("Too few levels (%d)", levelLimit)
	}
	if cutsPerLevel < 1 {
		glog.Fatalf("Too few cuts per level (%d)", cutsPerLevel)
	}
	var partitionsPerLevel int64 = int64(cutsPerLevel) + 1
	var totalLeaves int64 = 1
	for n := uint(0); n < (levelLimit - 2); n++ {
		totalLeaves *= partitionsPerLevel
	}
	totalLeaves += 4 // For the two special leaves at each of the first two levels.
	minSamples := totalLeaves * 100
	if int64(len(samples)) < minSamples {
		glog.Fatalf(
			"Too few location samples (%d); want at least %d samples given %d leaves in the partitioning",
			len(samples), minSamples, totalLeaves)
	}

	// Start the partitioning in the direction with the greatest distance.
	snLocations := append([]geo.Location(nil), samples...)
	geo.SortSouthToNorth(snLocations)

	southern, northern := getLocationsNearExtrema(snLocations)
	glog.Infof("southern position: %s", southern)
	glog.Infof("northern position: %s", northern)

	weLocations := samples
	samples = nil
	geo.SortWestToEast(weLocations)
	western, eastern := getLocationsNearExtrema(weLocations)
	glog.Infof("western position: %s", western)
	glog.Infof("eastern position: %s", eastern)

	var south, north, west, east geo.Location
	south.Lat = southern.Lat
	north.Lat = northern.Lat
	west.Lon = western.Lon
	east.Lon = eastern.Lon

	// Now use the median latitude and longitude for computing the
	// longitude and latitude distances, respectively.
	south.Lon = weLocations[len(weLocations)/2].Lon
	north.Lon = south.Lon
	west.Lat = snLocations[len(snLocations)/2].Lat
	east.Lat = west.Lat

	glog.Infof("south: %s", south)
	glog.Infof("north: %s", north)
	glog.Infof("west: %s", west)
	glog.Infof("east: %s", east)

	// Measure the distance.
	snDistance, _ := geo.ToDistanceAndHeading(south, north)
	weDistance, _ := geo.ToDistanceAndHeading(west, east)

	glog.Infof("snDistance: %v", snDistance)
	glog.Infof("weDistance: %v", weDistance)

	// TODO add func createRootPartitioners to create the 5 partitioners for
	// the two root levels.

	result := &AgencyPartitioner{Agency: agency}
	if weDistance >= snDistance {
		result.RootPartitioner = CreateWestEastPartitioners(
			-180, 180, -90, 90,
			0, levelLimit, int(cutsPerLevel),
			weLocations, true)
	} else {
		result.RootPartitioner = CreateSouthNorthPartitioners(
			-180, 180, -90, 90,
			0, levelLimit, int(cutsPerLevel),
			snLocations, false)
	}

	return result
}

func UnmarshalAgencyPartitioner(r io.Reader) *AgencyPartitioner {
	d := &Decoder{decoder: xml.NewDecoder(r)}
	d.Advance()
	a := &AgencyPartitioner{}
	a.decodeXml(d, 0)
	d.Next()
	if d.t != nil {
		glog.Warningf("Unexpected token(s) at end of partition index: %s", d)
	}
	return a
}
