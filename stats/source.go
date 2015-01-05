package stats

type Data1DSource interface {
	Len() int
	Value(n int) float64
	Weight(n int) float64
}
type Data1DSourceDelegate struct {
	L func() int
	V func(n int) float64
	W func(n int) float64
}

func (p *Data1DSourceDelegate) Len() int {
	return p.L()
}
func (p *Data1DSourceDelegate) Value(n int) float64 {
	return p.V(n)
}
func (p *Data1DSourceDelegate) Weight(n int) float64 {
	return p.W(n)
}

////////////////////////////////////////////////////////////////////////////////
type Data2DSource interface {
	Len() int
	X(n int) float64
	Y(n int) float64
	Weight(n int) float64
}
type Data2DSourceDelegate struct {
	Lf func() int
	Xf func(n int) float64
	Yf func(n int) float64
	Wf func(n int) float64
}

func (p *Data2DSourceDelegate) Len() int {
	return p.Lf()
}
func (p *Data2DSourceDelegate) X(n int) float64 {
	return p.Xf(n)
}
func (p *Data2DSourceDelegate) Y(n int) float64 {
	return p.Yf(n)
}
func (p *Data2DSourceDelegate) Weight(n int) float64 {
	return p.Wf(n)
}

////////////////////////////////////////////////////////////////////////////////

type SlidingWindowData2DSource struct {
	xs          []float64
	ys          []float64
	ws          []float64
	sampleLimit int
	nextIndex   int
}

func NewSlidingWindowData2DSource(sampleLimit int) *SlidingWindowData2DSource {
	return &SlidingWindowData2DSource{
		xs:          make([]float64, 0, sampleLimit),
		ys:          make([]float64, 0, sampleLimit),
		ws:          make([]float64, 0, sampleLimit),
		sampleLimit: sampleLimit,
		nextIndex:   -1,
	}
}

func (p *SlidingWindowData2DSource) SampleLimit() int {
	return p.sampleLimit
}

func (p *SlidingWindowData2DSource) AddSample(x, y, w float64) {
	ndx := p.nextIndex
	if ndx >= 0 {
		p.xs[ndx] = x
		p.ys[ndx] = y
		p.ws[ndx] = w
		ndx++
		if ndx >= p.sampleLimit {
			ndx = 0
		}
		p.nextIndex = ndx
		return
	}

	p.xs = append(p.xs, x)
	p.ys = append(p.ys, y)
	p.ws = append(p.ws, w)
	if len(p.xs) >= p.sampleLimit {
		p.nextIndex = 0
	}
}

func (p *SlidingWindowData2DSource) Len() int {
	return len(p.xs)
}

func (p *SlidingWindowData2DSource) X(n int) float64 {
	return p.xs[n]
}

func (p *SlidingWindowData2DSource) Y(n int) float64 {
	return p.ys[n]
}

func (p *SlidingWindowData2DSource) Weight(n int) float64 {
	return p.ws[n]
}
