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
