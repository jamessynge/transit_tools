package nblocations

import (
	"github.com/golang/glog"

	"github.com/jamessynge/transit_tools/geo"
	"github.com/jamessynge/transit_tools/util"
)

////////////////////////////////////////////////////////////////////////////////

type Stub interface {
//	IsLeaf() bool
//	SplitSouthNorth(cuts int) *SNNode
//	SplitWestEast(cuts int) *WENode
	Region() geo.Rect
	Level() uint
	Area() geo.MetersSq
	Parent() DirectionalStub
	FindLeaves(leaves []*LeafNode) []*LeafNode
	ConvertToPartitioner(level uint) Partitioner
}
type DirectionalStub interface {
	Stub
	ReplaceLeaf(leaf *LeafNode, stub Stub)
	NumChildren() int
}

type Node struct {
	region		geo.Rect
	level         uint // 0-based, <= *partitionsLevelsFlag
	area geo.MetersSq
	parent DirectionalStub
}
func (p *Node) Region() geo.Rect {
	return p.region
}
func (p *Node) Level() uint {
	return p.level
}
func (p *Node) Area() geo.MetersSq {
	return p.area
}
func (p *Node) Parent() DirectionalStub {
	return p.parent
}

// Not a partitioner, but rather a stub used to create GridNodes if split,
// or LeafPartitioners if not split.
type LeafNode struct {
	Node
	sortedLocations []geo.Location
	sortedWestToEast bool
}
func (p *LeafNode) IsLeaf() bool {
	return true
}
func (p *LeafNode) FindLeaves(leaves []*LeafNode) []*LeafNode {
	return append(leaves, p)
}

type SplitCutsFn func(leaf *LeafNode) int

func (p *LeafNode) SplitSouthNorth(policy SplitCutsFn) Stub {
	cuts := policy(p)
	if cuts < 1 {
		return p
	}
	// Split the samples (sortedLocations) into cuts+1 subsets.
	sortedLocations := p.sortedLocations
	if p.sortedWestToEast {
		sortedLocations = make([]geo.Location, len(p.sortedLocations))
		copy(sortedLocations, p.sortedLocations)
		geo.SortSouthToNorth(sortedLocations)
	}
	delta := p.region.DeltaLatitude()
	step := delta / geo.Latitude(cuts + 1)
	cutAtLat := p.region.South
	souths := []geo.Latitude{cutAtLat}
	norths := []geo.Latitude{}
	splitPoints := []int{}
	for n := 1; n <= cuts; n++ {
		cutAtLat += step
		norths = append(norths, cutAtLat)
		souths = append(souths, cutAtLat)
		splitPoints = append(splitPoints, getIndexOfLatitude(sortedLocations, cutAtLat))
	}
	norths = append(norths, p.region.North)
	kidsSamples := splitLocationsByIndices(sortedLocations, splitPoints)
	// Create the result and its children (leaves).
	result := &SNNode{
		DirectionalNode: DirectionalNode{
			Node: p.Node,
			children: make([]Stub, cuts+1),
		},
	}
	for n := range kidsSamples {
		kid := &LeafNode{
			Node: Node{
				region: geo.Rect{
					South: souths[n],
					North: norths[n],
					West: p.region.West,
					East: p.region.East,
				},
				level: p.level + 1,
				parent: result,
			},
			sortedLocations: kidsSamples[n],
			sortedWestToEast: false,
		}
		kid.area = kid.region.Area()
		result.children[n] = kid.SplitWestEast(policy)
	}
	return result
}
func (p *LeafNode) SplitWestEast(policy SplitCutsFn) Stub {
	cuts := policy(p)
	if cuts < 1 {
		return p
	}
	// Split the samples (sortedLocations) into cuts+1 subsets.
	sortedLocations := p.sortedLocations
	if !p.sortedWestToEast {
		sortedLocations = make([]geo.Location, len(p.sortedLocations))
		copy(sortedLocations, p.sortedLocations)
		geo.SortWestToEast(sortedLocations)
	}
	delta := p.region.DeltaLongitude()
	step := delta / geo.Longitude(cuts + 1)
	cutAtLon := p.region.West
	wests := []geo.Longitude{cutAtLon}
	easts := []geo.Longitude{}
	splitPoints := []int{}
	for n := 1; n <= cuts; n++ {
		cutAtLon += step
		easts = append(easts, cutAtLon)
		wests = append(wests, cutAtLon)
		splitPoints = append(splitPoints, getIndexOfLongitude(sortedLocations, cutAtLon))
	}
	easts = append(easts, p.region.East)
	kidsSamples := splitLocationsByIndices(sortedLocations, splitPoints)
	// Create the result and its children (leaves).
	result := &WENode{
		DirectionalNode: DirectionalNode{
			Node: p.Node,
			children: make([]Stub, cuts+1),
		},
	}
	for n := range kidsSamples {
		kid := &LeafNode{
			Node: Node{
				region: geo.Rect{
					South: p.region.South,
					North: p.region.North,
					West: wests[n],
					East: easts[n],
				},
				level: p.level + 1,
				parent: result,
			},
			sortedLocations: kidsSamples[n],
			sortedWestToEast: true,
		}
		kid.area = kid.region.Area()
		result.children[n] = kid.SplitSouthNorth(policy)
	}
	return result
}

func (p *LeafNode) ConvertToPartitioner(level uint) Partitioner {
	if glog.V(1) {
		util.IndentedInfof("LeafNode.ConvertToPartitioner(%d) %s", level, p.Region())
	}
	return CreateLeafPartitioner(
		p.region.West, p.region.East, p.region.South, p.region.North, level)
}

type DirectionalNode struct {
	Node
	children []Stub
}
func (p *DirectionalNode) IsLeaf() bool {
	return false
}
func (p *DirectionalNode) FindLeaves(leaves []*LeafNode) []*LeafNode {
	for _, kid := range p.children {
		leaves = kid.FindLeaves(leaves)
	}
	return leaves
}
func (p *DirectionalNode) ReplaceLeaf(leaf *LeafNode, stub Stub) {
	var target Stub = leaf
	for n, kid := range p.children {
		if kid == target {
			p.children[n] = stub
			return
		}
	}
}
func (p *DirectionalNode) NumChildren() int {
	return len(p.children)
}

type SNNode struct {
	DirectionalNode
}
func (p *SNNode) ConvertToPartitioner(level uint) Partitioner {
	if glog.V(1) {
		defer util.EnterExitInfof("SNNode.ConvertToPartitioner(%d) %s",
															level, p.Region())()
	}
	result := &SouthNorthPartitioners{
		PartitionsBase: PartitionsBase{
			RegionBase: RegionBase{
				West:  p.region.West,
				East:  p.region.East,
				South: p.region.South,
				North: p.region.North,
			},
			level:    level,
			minCoord: float64(p.region.South),
			maxCoord: float64(p.region.North),
		},
	}
	for ndx, kid := range p.children {
		result.SubPartitions = append(
				result.SubPartitions, kid.ConvertToPartitioner(level + 1))
		if ndx == 0 { continue }
		result.cutPoints = append(result.cutPoints, float64(kid.Region().South))
	}
	return result
}

type WENode struct {
	DirectionalNode
}
func (p *WENode) ConvertToPartitioner(level uint) Partitioner {
	if glog.V(1) {
		defer util.EnterExitInfof("WENode.ConvertToPartitioner(%d) %s",
															level, p.Region())()
	}
	result := &WestEastPartitioners{
		PartitionsBase: PartitionsBase{
			RegionBase: RegionBase{
				West:  p.region.West,
				East:  p.region.East,
				South: p.region.South,
				North: p.region.North,
			},
			level:    level,
			minCoord: float64(p.region.West),
			maxCoord: float64(p.region.East),
		},
	}
	for ndx, kid := range p.children {
		result.SubPartitions = append(
				result.SubPartitions, kid.ConvertToPartitioner(level + 1))
		if ndx == 0 { continue }
		result.cutPoints = append(result.cutPoints, float64(kid.Region().West))
	}
	return result
}

////////////////////////////////////////////////////////////////////////////////
// Given an unordered collection of samples, determine the approximate
// bounds of the transit region (lat-lon rectangle), and then choose a
// size (in meters) for squares into which we'll divide the transit
// region, and create Stubs for those squares.
func createRootStub(
		samples []geo.Location, minSquareSide geo.Meters,
		maxArea geo.MetersSq, maxSamplesFraction float64) Stub {
	// What is the maximum number of samples we want in a region.
	maxSamples := int(float64(len(samples)) * maxSamplesFraction)

	minimumArea := geo.MetersSq(minSquareSide * minSquareSide)
	minAreaToQuarter := minimumArea * 4

	// Determine the basic transit region.
	snLocations := append([]geo.Location(nil), samples...)
	geo.SortSouthToNorth(snLocations)

	southern, northern := getLocationsNearExtrema(snLocations)
//	glog.Infof("southern position: %s", southern)
//	glog.Infof("northern position: %s", northern)

	weLocations := samples
	samples = nil
	geo.SortWestToEast(weLocations)
	western, eastern := getLocationsNearExtrema(weLocations)
//	glog.Infof("western position: %s", western)
//	glog.Infof("eastern position: %s", eastern)

	baseRegion := geo.Rect{
		South: southern.Lat,
		North: northern.Lat,
		West: western.Lon,
		East: eastern.Lon,
	}
	glog.Infof("baseRegion: %s", baseRegion)

	snDistance := baseRegion.Height()
	area := baseRegion.Area()
	weDistance := geo.Meters(float64(area) / float64(snDistance))

	glog.Infof("base dims: SN: %v km  X  %v km", snDistance / 1000, weDistance / 1000)

	candidate, err := choosePartitionSize(
			float64(snDistance * 1), float64(snDistance * 1.20),
			float64(weDistance * 1), float64(weDistance * 1.20),
			float64(minSquareSide * 16), float64(minSquareSide))
	if err != nil {
		glog.Fatal(err)
	}

	snDistance = geo.Meters(candidate.snDistance)
	weDistance = geo.Meters(candidate.weDistance)
	glog.Infof("transit dims: SN: %v km  X  %v km", snDistance / 1000, weDistance / 1000)

	snParts := candidate.snDistance / candidate.unit
	weParts := candidate.weDistance / candidate.unit
	if snParts == 1 || weParts == 1 {
		snParts *= 2
		weParts *= 2
	}
	snCuts := snParts - 1
	weCuts := weParts - 1

	// Determine the region for the root based on the center of the baseRegion
	// and the selected candidate.
	center := baseRegion.Center()
	region := center.RectCenteredAt(snDistance, weDistance)

	rootLeaf := &LeafNode{
		Node: Node{
			region: region,
			level: 0,
		},
	}
	rootLeaf.area = rootLeaf.region.Area()

	basePolicy := func(leaf *LeafNode) int {
		if leaf.level % 2 == 1 {
			// Split odd numbered levels 3, 5, 7, etc. the same as their immediate
			// parent.
			return leaf.Parent().NumChildren() - 1
		}
		area := leaf.Area()
		if area > maxArea {
			return 1
		}
		if area >= minAreaToQuarter && len(leaf.sortedLocations) > maxSamples {
			return 1
		}
		return 0
	}

	if snCuts >= weCuts {
		rootLeaf.sortedLocations = snLocations
		rootLeaf.sortedWestToEast = false
		policy := func(leaf *LeafNode) int {
			level := leaf.Level()
			if level == 0 {
				return snCuts
			} else if level == 1 {
				return weCuts
			}
			return basePolicy(leaf)
		}
		return rootLeaf.SplitSouthNorth(policy)
	} else {
		rootLeaf.sortedLocations = weLocations
		rootLeaf.sortedWestToEast = true
		policy := func(leaf *LeafNode) int {
			level := leaf.Level()
			if level == 0 {
				return weCuts
			} else if level == 1 {
				return snCuts
			}
			return basePolicy(leaf)
		}
		return rootLeaf.SplitWestEast(policy)
	}
}

// Create partitioners such that the resulting partition
// files are of roughly equal size (number of records).
func CreateAgencyPartitioner3(
		agency string, samples []geo.Location,
		minSquareSide geo.Meters, maxSquareSide geo.Meters,
		maxSamplesFraction float64) *AgencyPartitioner {
	maxArea := geo.MetersSq(maxSquareSide * maxSquareSide)
	stub := createRootStub(
			samples, minSquareSide, maxArea, maxSamplesFraction)

	// Create two levels of partitioners for the area outside of
	// the transit region.  Level 2 is 3 regions forming a band all the way
	// around the earth, between latitudes tr.South and tr.North.
	tr := stub.Region()
	westOfRegion := CreateLeafPartitioner(-180, tr.West, tr.South, tr.North, 2)
	theRegion := stub.ConvertToPartitioner(2)
	eastOfRegion := CreateLeafPartitioner(tr.East, 180, tr.South, tr.North, 2)

	// Form those 3 regions into a single region, a band around the earth.
	regionBand := &WestEastPartitioners{
		PartitionsBase: PartitionsBase{
			RegionBase: RegionBase{
				West:  -180,
				East:  180,
				South: tr.South,
				North: tr.North,
			},
			level:    1,
			minCoord: -180,
			maxCoord: 180,
		},
	}
	regionBand.SubPartitions = append(regionBand.SubPartitions, westOfRegion)
	regionBand.cutPoints = append(regionBand.cutPoints, float64(tr.West))
	regionBand.SubPartitions = append(regionBand.SubPartitions, theRegion)
	regionBand.cutPoints = append(regionBand.cutPoints, float64(tr.East))
	regionBand.SubPartitions = append(regionBand.SubPartitions, eastOfRegion)

	// Level 1 is 3 regions, a southern and northern cap on either side of
	// the level 2 band, plus that band.
	southOfRegion := CreateLeafPartitioner(-180, 180, -90, tr.South, 1)
	northOfRegion := CreateLeafPartitioner(-180, 180, tr.North, 90, 1)

	root := &SouthNorthPartitioners{
		PartitionsBase: PartitionsBase{
			RegionBase: RegionBase{
				West:  -180,
				East:  180,
				South: -90,
				North: 90,
			},
			minCoord: float64(-90),
			maxCoord: float64(90),
			level:    0,
		},
	}
	root.SubPartitions = append(root.SubPartitions, southOfRegion)
	root.cutPoints = append(root.cutPoints, float64(tr.South))
	root.SubPartitions = append(root.SubPartitions, regionBand)
	root.cutPoints = append(root.cutPoints, float64(tr.North))
	root.SubPartitions = append(root.SubPartitions, northOfRegion)

	return &AgencyPartitioner{Agency: agency, RootPartitioner: root}
}
