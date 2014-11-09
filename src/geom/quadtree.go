package geom

import (
	"fmt"
	"math"
)

// This is the minimal interface that needs to be implemented
// in order to be inserted into the quadtree.
type IntersectBounder interface {
	// Returns the axis aligned bounding box of the intersection between the
	// shape that this object represents and the axis aligned rectangle r.
	// Since the underlying shape may not be an axis aligned rectangle,
	// this method enables the implementation to return a much smaller
	// rectangle, or even to indicate that there is no overlap.
	// This may be called many times as an object is inserted into a quad tree,
	// when a quad tree leaf node is split, and when searching for objects inside
	// a rectangle.
	IntersectBounds(r Rect) (intersection Rect, empty bool)
}

type Intersecter interface {
	// Does the object's shape overlap the specified axis aligned rectangle?
	// The default implementation used by QuadTree simply calls IntersectBounds,
	// returning !empty, but if there is a more efficient approach to answering
	// the question, this interface allows for it to be provided.
	Intersects(r Rect) bool
}

// If an object to be inserted into a quadtree is actually inserted as several
// different objects (e.g. a path composed of segments, inserted as one object
// per segment), this interface allows
type UniqueIder interface {
	// Returns a value used to determine, during a Visit, whether the object
	// has already been visited (i.e. an underlying object may be split into
	// multiple QuadTreeDatum instances, or the datum may have been inserted into
	// multiple leaves, or the object may be represented by just one datum, in
	// which case the return value of UniqueId can just be the datum itself).
	UniqueId() interface{}
}

//
//type IntersectsBounder interface {
//	IntersectBounder
//	Intersecter
//}
//
//type UniqueIdIntersectBounder interface {
//	IntersectBounder
//	UniqueIder
//}

type QuadTreeDatum interface {
	IntersectBounder
	Intersecter
	UniqueIder
}

type QuadTreeVisitor interface {
	Visit(datum IntersectBounder)
}

type QuadTree interface {
	Bounds() Rect

	Insert(datum IntersectBounder) (err error)

	//	Remove(datum QuadTreeDatum) bool

	Visit(bounds Rect, visitor QuadTreeVisitor)

	// Reduce the bounds of the quadtree based on the current contents.
	NarrowBounds()
}

////////////////////////////////////////////////////////////////////////////////

const (
	kSplitThreshold = 32
)

type quadtree struct {
	root qtNode
}

type qtSearcher struct {
	bounds         *Rect
	alreadyVisited map[interface{}]bool
	visitor        QuadTreeVisitor
}

type qtNode interface {
	Bounds() *Rect
	Center() Point
	Level() int
	// bounds is the subset of datum.bounds within this node
	Insert(bounds *Rect, datum QuadTreeDatum)
	//	Remove(bounds *Rect, datum QuadTreeDatum) bool
	Visit(searcher *qtSearcher)
	ReadyToSplit() bool
	Split() qtNode // Create a new interior node with one or more leaves

	// If all of the data is in one quadrant, runs NarrowBounds on that
	// quadrant, else returns itself.
	NarrowBounds() qtNode
}

type qtNodeCommon struct {
	bounds          Rect
	center          Point
	level           int
	data            []*qtData
	numNotSplitable int
}

type qtLeaf struct {
	qtNodeCommon
	checkedSizeForSplit bool
	okToSplit           bool
}

type qtNonLeaf struct {
	qtNodeCommon
	kids [4]qtNode
}

type qtData struct {
	bounds Rect // Bounds within the containing qtNode; may be a subset of data.Bounds.
	datum  QuadTreeDatum
}

type qtDatumDelegate struct {
	IntersectBounder
	Intersecter
	UniqueIder
}

type qtIntersectDelegate struct {
	IntersectBounder
}

type qtUniqueIdDelegate struct {
	IntersectBounder
}

////////////////////////////////////////////////////////////////////////////////

func NewQuadTree(bounds Rect) QuadTree {
	leaf := newLeaf(&bounds, 0)
	return &quadtree{root: leaf}
}

func (t *quadtree) Bounds() Rect {
	return *t.root.Bounds()
}

func (t *quadtree) Insert(ib IntersectBounder) (err error) {
	if datum, ok := ib.(QuadTreeDatum); ok {
		return t.InsertFull(datum)
	}
	datum := &qtDatumDelegate{}
	datum.IntersectBounder = ib
	if i, ok := ib.(Intersecter); ok {
		datum.Intersecter = i
	} else {
		datum.Intersecter = &qtIntersectDelegate{ib}
	}
	if u, ok := ib.(UniqueIder); ok {
		datum.UniqueIder = u
	} else {
		datum.UniqueIder = &qtUniqueIdDelegate{ib}
	}
	return t.InsertFull(datum)
}

func (t *quadtree) InsertFull(datum QuadTreeDatum) (err error) {
	r, empty := datum.IntersectBounds(*t.root.Bounds())
	if empty {
		err = fmt.Errorf("datum is not in quadtree region")
	}
	bounds := new(Rect)
	*bounds = r
	t.root.Insert(bounds, datum)
	if t.root.ReadyToSplit() {
		t.root = t.root.Split()
	}
	return
}

//func (t *quadtree) Remove(datum QuadTreeDatum) bool {
//	panic("not yet implemented")
//}

func (t *quadtree) Visit(bounds Rect, visitor QuadTreeVisitor) {
	s := qtSearcher{bounds: &bounds, visitor: visitor}
	s.alreadyVisited = make(map[interface{}]bool)
	t.root.Visit(&s)
}

func (t *quadtree) NarrowBounds() {
	t.root = t.root.NarrowBounds()
}

////////////////////////////////////////////////////////////////////////////////

func (p *qtIntersectDelegate) Intersects(r Rect) bool {
	_, empty := p.IntersectBounds(r)
	return !empty
}

func (p *qtUniqueIdDelegate) UniqueId() interface{} {
	return p.IntersectBounder
}

////////////////////////////////////////////////////////////////////////////////

func (n *qtNodeCommon) Bounds() *Rect {
	return &n.bounds
}

func (n *qtNodeCommon) Center() Point {
	return n.center
}

func (n *qtNodeCommon) Level() int {
	return n.level
}

func isStrictSuperSet(a, b *Rect) bool {
	// Common case is that the a contains b, and is b is smaller than a, and
	// doesn't touch a boundary.
	if a.MinX < b.MinX && b.MaxX < a.MaxX &&
		a.MinY < b.MinY && b.MaxY < a.MaxY {
		return true
	}
	// It's a programming bug (not by the QuadTree caller) if b not a subset of a.
	if b.MinX < a.MinX && a.MaxX < b.MaxX &&
		b.MinY < a.MinY && a.MaxY < b.MaxY {
		panic(fmt.Errorf("a doesn't contain b:\na: %#v\nb: %#v", *a, *b))
	}
	return *a != *b
}

func (n *qtNodeCommon) AddLocal(bounds *Rect, datum QuadTreeDatum) {
	n.data = append(n.data, &qtData{*bounds, datum})
	if bounds.IsPoint() || isStrictSuperSet(&n.bounds, bounds) {
		// datum can be be pushed down to kids.
		return
	}
	n.numNotSplitable++
}

//func (n *qtNodeCommon) RemoveLocal(bounds *Rect, datum QuadTreeDatum) bool {
//	panic("not yet implemented")
//}

func (n *qtNodeCommon) VisitLocal(s *qtSearcher) {
	for _, d := range n.data {
		s.maybeVisit(d)
	}
}

func (n *qtNodeCommon) IsLeftOrRight(bounds *Rect) (leftOnly, rightOnly bool) {
	if n.center.X <= bounds.MinX {
		rightOnly = true
	} else if bounds.MaxX <= n.center.X {
		leftOnly = true
	}
	return
}

func (n *qtNodeCommon) IsLowerOrUpper(bounds *Rect) (lowerOnly, upperOnly bool) {
	if n.center.Y <= bounds.MinY {
		upperOnly = true
	} else if bounds.MaxY <= n.center.Y {
		lowerOnly = true
	}
	return
}

////////////////////////////////////////////////////////////////////////////////

func newLeaf(bounds *Rect, level int) *qtLeaf {
	//	fmt.Printf("Creating leaf at level %d: %v\n", level, *bounds)
	return &qtLeaf{
		qtNodeCommon: qtNodeCommon{
			bounds: *bounds,
			center: bounds.Center(),
			level:  level,
		},
	}
}

// bounds is the subset of datum.bounds within this node
func (n *qtLeaf) Insert(bounds *Rect, datum QuadTreeDatum) {
	n.AddLocal(bounds, datum)
}

//func (n *qtLeaf) Remove(bounds *Rect, datum QuadTreeDatum) bool {
//	return n.RemoveLocal(bounds, datum)
//}

func (n *qtLeaf) Visit(s *qtSearcher) {
	n.VisitLocal(s)
}

const smallestLeaf = math.SmallestNonzeroFloat64 * 16

func (n *qtLeaf) ReadyToSplit() bool {
	numSplitable := len(n.data) - n.numNotSplitable
	if numSplitable <= kSplitThreshold {
		return false
	}
	// Have enough data to split, but avoid
	// splitting so deeply that we lose precision.
	if !n.checkedSizeForSplit {
		n.checkedSizeForSplit = true
		xn := math.Nextafter(n.bounds.MinX, n.bounds.MaxX)
		xn = math.Nextafter(xn, n.bounds.MaxX)
		yn := math.Nextafter(n.bounds.MinY, n.bounds.MaxY)
		yn = math.Nextafter(yn, n.bounds.MaxY)
		if xn < n.center.X && yn < n.center.Y {
			n.okToSplit = true
		} else {
			n.okToSplit = false
			fmt.Printf("Not splitting degenerate node with %d data at level %d\n",
				numSplitable, n.level)
			fmt.Printf("Node bounds: %v\n\n", n.bounds)
			fmt.Printf("xn: %v\n", xn)
			fmt.Printf("yn: %v\n", yn)
			fmt.Printf("n.bounds.MinX+xn*4: %v\n", n.bounds.MinX+xn*4)
			fmt.Printf("n.bounds.MinY+yn*4: %v\n", n.bounds.MinY+yn*4)
		}
	}
	return n.okToSplit
}

func (n *qtLeaf) Split() qtNode {
	newNode := &qtNonLeaf{
		qtNodeCommon: qtNodeCommon{
			bounds: *n.Bounds(),
			center: n.Center(),
			level:  n.Level(),
		},
	}
	for _, d := range n.data {
		newNode.Insert(&d.bounds, d.datum)
	}
	return newNode
}

func (n *qtLeaf) NarrowBounds() qtNode {
	return n
}

////////////////////////////////////////////////////////////////////////////////

func newNonLeaf(bounds *Rect, level int) *qtNonLeaf {
	return &qtNonLeaf{
		qtNodeCommon: qtNodeCommon{
			bounds: *bounds,
			center: bounds.Center(),
			level:  level,
		},
	}
}

func (n *qtNonLeaf) AddQuadrantLeaf(quadrant int) {
	if n.kids[quadrant] != nil {
		panic(fmt.Errorf("quadrant %v already exists: %#v", quadrant, *n))
	}
	b := n.bounds
	switch quadrant {
	case kUpperLeft:
		b.MaxX = n.center.X
		b.MinY = n.center.Y
	case kUpperRight:
		b.MinX = n.center.X
		b.MinY = n.center.Y
	case kLowerLeft:
		b.MaxX = n.center.X
		b.MaxY = n.center.Y
	case kLowerRight:
		b.MinX = n.center.X
		b.MaxY = n.center.Y
	default:
		panic(fmt.Errorf("invalid quadrant: %v", quadrant))
	}
	n.kids[quadrant] = newLeaf(&b, n.level+1)
}

func (n *qtNonLeaf) InsertInQuadrant(bounds *Rect, datum QuadTreeDatum,
	quadrant int) (inserted bool) {
	kid := n.kids[quadrant]
	if kid == nil {
		n.AddQuadrantLeaf(quadrant)
		kid = n.kids[quadrant]
	}
	if bounds == nil {
		intersection, empty := datum.IntersectBounds(*kid.Bounds())
		if empty {
			return false
		}
		bounds = &intersection
	}
	kid.Insert(bounds, datum)
	if kid.ReadyToSplit() {
		n.kids[quadrant] = kid.Split()
	}
	return true
}

// bounds is the subset of datum.bounds within this node
func (n *qtNonLeaf) Insert(bounds *Rect, datum QuadTreeDatum) {
	if n.bounds == *bounds {
		// No point in subdividing.
		n.qtNodeCommon.AddLocal(bounds, datum)
		return
	}
	// Commonly, a datum will belong in only one quadrant,
	// so optimize for that case.
	leftOnly, rightOnly := n.IsLeftOrRight(bounds)
	lowerOnly, upperOnly := n.IsLowerOrUpper(bounds)
	if leftOnly {
		if lowerOnly {
			n.InsertInQuadrant(bounds, datum, kLowerLeft)
			return
		} else if upperOnly {
			n.InsertInQuadrant(bounds, datum, kUpperLeft)
			return
		}
		// It spans upper and lower.
		n.InsertInQuadrant(nil, datum, kLowerLeft)
		n.InsertInQuadrant(nil, datum, kUpperLeft)
		return
	} else if rightOnly {
		if lowerOnly {
			n.InsertInQuadrant(bounds, datum, kLowerRight)
			return
		} else if upperOnly {
			n.InsertInQuadrant(bounds, datum, kUpperRight)
			return
		}
		// It spans upper and lower.
		n.InsertInQuadrant(nil, datum, kLowerRight)
		n.InsertInQuadrant(nil, datum, kUpperRight)
		return
	} else if lowerOnly {
		n.InsertInQuadrant(nil, datum, kLowerLeft)
		n.InsertInQuadrant(nil, datum, kLowerRight)
		return
	} else if upperOnly {
		n.InsertInQuadrant(nil, datum, kUpperLeft)
		n.InsertInQuadrant(nil, datum, kUpperRight)
		return
	}
	// Need to split across all quadrants.
	n.InsertInQuadrant(nil, datum, kLowerLeft)
	n.InsertInQuadrant(nil, datum, kUpperLeft)
	n.InsertInQuadrant(nil, datum, kLowerRight)
	n.InsertInQuadrant(nil, datum, kUpperRight)
}

//func (n *qtNonLeaf) Remove(bounds *Rect, datum QuadTreeDatum) (removed bool) {
//	removed = n.qtNodeCommon.RemoveLocal(bounds, datum)
//	for _, k := range n.kids {
//		if k.Remove(bounds, datum) {
//			removed = true
//		}
//	}
//	return
//}

func (n *qtNonLeaf) Visit(s *qtSearcher) {
	n.VisitLocal(s)
	for _, k := range n.kids {
		if k != nil {
			k.Visit(s)
		}
	}
}

func (n *qtNonLeaf) ReadyToSplit() bool {
	return false
}

func (n *qtNonLeaf) Split() qtNode {
	return n
}

func (n *qtNonLeaf) NarrowBounds() qtNode {
	if len(n.data) > 0 {
		return n
	}
	var firstKid qtNode
	for _, k := range n.kids {
		if k != nil {
			if firstKid != nil {
				return n
			}
			firstKid = k
		}
	}
	if firstKid != nil {
		return firstKid.NarrowBounds()
	}
	// Unusual case: we have a non-leaf with no data.
	return n
}

//func DatumBoundsIntersect(datum QuadTreeDatum, bounds *Rect) bool {
//	_, empty := datum.IntersectBounds(*bounds)
//	return !empty
//}

// If we haven't already visited d.datum and d.bounds overlaps with s.bounds,
// call visitor.Visit.
func (s *qtSearcher) maybeVisit(d *qtData) {
	id := d.datum.UniqueId()
	if s.alreadyVisited[id] == true {
		// Already visited.
		return
	}
	// Compare the datum's bounds within the current qtnode with the
	// search bounds.
	if !d.bounds.IntersectsP(s.bounds) {
		// Definitely not overlapping.
		return
	}
	// Now a potentially more expensive test: ask the datum if it overlaps the
	// search bounds.
	if !d.datum.Intersects(*s.bounds) {
		// Doesn't really overlap.
		return
	}
	// Overlaps.
	s.alreadyVisited[id] = true
	s.visitor.Visit(d.datum)
}

////////////////////////////////////////////////////////////////////////////////

/*



type qtNode interface {
	Bounds() *Rect
	Center() Point
	// bounds is the subset of datum.bounds within this node
	Insert(bounds *Rect, datum QuadTreeDatum)
	Remove(bounds *Rect, datum QuadTreeDatum) bool
	Visit(searcher *qtSearcher)
	ReadyToSplit() bool
	Split() *qtNode  // Create a new interior node with one or more leaves
}













func NewQuadTree(bounds Rect) QuadTree {
	return &quadtree{root:newQTNode(bounds)}
}

func (t *quadtree) Bounds() Rect {
	return t.root.bounds
}

func (t *quadtree) Insert(datum QuadTreeDatum) (err error) {
	bounds := datum.Bounds()
	if !t.root.bounds.Contains(bounds) {
		err = fmt.Errorf("%#v is not inside bounds %#v", bounds, t.root.bounds)
		return
	}
	err = nil
	if bounds.IsPoint() {
		t.root.InsertPoint(&bounds, datum)
	} else {
		t.root.InsertArea(&bounds, sd)
	}
	return
}

func (t *quadtree) Remove(datum QuadTreeDatum) bool {
	return false
}

func (t *quadtree) Visit(bounds Rect, visitor QuadTreeVisitor) {
	pb := &bounds
	if t.root.Intersects(pb) {
		t.root.visit(pb, visitor)
	}
}

type qtnode struct {
	bounds  Rect
	center  Point
	level   uint
	hasKids	bool
	kids    [4]*qtnode
	unsplitableData *qtdata
	splitableData *qtsdata
	splitableCount uint
}


func newQTNode(bounds Rect) *qtnode {
	return &qtnode{bounds: bounds, center: bounds.Center()}
}

func (n *qtnode) Intersects(r *Rect) bool {
	return r.IntersectsP(&n.bounds)
}

func (n *qtnode) Contains(r *Rect) bool {
	return n.bounds.Contains(*r)
}

func isSplitable(datum QuadTreeDatum) bool {
	_, ok := datum.(QuadTreeSplitableDatum)
	return ok
}

// Which quadrants of the node does the rectangle r overlap?
func (n *qtnode) OverlappingQuadrants(bounds *Rect) (
		upperLeft, upperRight, lowerLeft, lowerRight bool,
		count byte) {
	cx, cy := n.center.X, n.center.Y
	if cx < bounds.MaxX {
		// Overlaps on the right.

	if r.MinX >= p.X {
		// Right only
		if r.MinY >= p.Y {
			// Upper-Right only
			return false
		} else if r.MaxY < p.Y {
			// Lower-Right only
			return false
		}
		return true
	} else if r.MaxX < p.X {
		// Left only
		if r.MinY >= p.Y {
			// Upper-Left only
			return false
		} else if r.MaxY < p.Y {
			// Lower-Left only
			return false
		}
		return true
	} else {
		return true
	}



// Given the axes through a point (i.e. the line X=p.X and the line Y=p.Y),
// does one or both of those run through the rect?  Used to determine if
// a datum (with bounds r) is in more than one child.
func containsAnAxis(r *Rect, p *Point) bool {
	if r.MinX >= p.X {
		// Right only
		if r.MinY >= p.Y {
			// Upper-Right only
			return false
		} else if r.MaxY < p.Y {
			// Lower-Right only
			return false
		}
		return true
	} else if r.MaxX < p.X {
		// Left only
		if r.MinY >= p.Y {
			// Upper-Left only
			return false
		} else if r.MaxY < p.Y {
			// Lower-Left only
			return false
		}
		return true
	} else {
		return true
	}






	// Does the segment belong in a child node?
	if newSeg.bounds.MaxX <= t.center.X {
		if newSeg.bounds.MaxY <= t.center.Y {
			return t.insertSegmentInQuadrant(kLowerLeft, newSeg)
		} else if newSeg.bounds.MinY >= t.center.Y {
			return t.insertSegmentInQuadrant(kUpperLeft, newSeg)
		}
	}
	if newSeg.bounds.MinX >= t.center.X {
		if newSeg.bounds.MaxY <= t.center.Y {
			return t.insertSegmentInQuadrant(kLowerRight, newSeg)
		} else if newSeg.bounds.MinY >= t.center.Y {
			return t.insertSegmentInQuadrant(kUpperRight, newSeg)
		}
	}

	if r.IsPoint() {
		return r.MinX == p.X && r.MinY == p.Y
	}
	return r.MinX <= p.X && p.X < r.MaxX &&
	       r.MinY <= p.Y && p.Y < r.MaxY
}

func (n *qtnode) SpansCenter(bounds *Rect) bool {

	_, ok := datum.(QuadTreeSplitableDatum)
	return ok
}




// Caller has ensured that this node CONTAINS bounds.
func (n *qtnode) Insert(bounds *Rect, datum QuadTreeDatum) {
	if n.hasKids {
		for _, kid := range n.kids {
			if kid != nil && kid.Contains(bounds) {
				kid.Insert(bounds, datum)
				return
			}
		}
		if bounds.IsPoint() {
			// Can't span multiple kids, so need to make a child and insert
			n.MakeKids(bounds)
			n.Insert(bounds, datum) // Tail recursion, which should now be able to insert.
			return
		}
		// The datum MAY span multiple kids, or the appropriate kid doesn't exist.
		if


	}
	// Either there are no kids, or the datum's bounds span multiple kids.
	if



	if bounds.IsPoint() {
		// Can't span multiple kids, so need to make a child.


}

// Caller has ensured that bounds intersects this node.
func (n *qtnode) visit(bounds *Rect, visitor QuadTreeVisitor) {
	for pd := n.unsplitableData; pd != nil; pd = pd.next {
		if bounds.IntersectsP(&pd.bounds) {
			visitor.Visit(pd.data)
		}
	}
	for pd := n.splitableData; pd != nil; pd = pd.next {
		if bounds.IntersectsP(&pd.bounds) {
			visitor.Visit(pd.data)
		}
	}
	for _, kid := range n.kids {
		if kid != nil && bounds.IntersectsP(&kid.bounds) {
			kid.visit(bounds, visitor)
		}
	}
}

func newQTData(data QuadTreeDatum) *qtdata {
	return &qtdata{
		bounds: data.Bounds(),
		data: data,
	}
}

func newSQTData(data QuadTreeSplitableDatum) *qtsdata {
	return &qtsdata{
		bounds: data.Bounds(),
		data: data,
	}
}
*/
