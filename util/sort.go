package util

import (
	"sort"
)

type sortDelegate struct {
	n    int
	less func(i, j int) bool
	swap func(i, j int)
}

func (p *sortDelegate) Len() int {
	return p.n
}
func (p *sortDelegate) Less(i, j int) bool {
	return p.less(i, j)
}
func (p *sortDelegate) Swap(i, j int) {
	p.swap(i, j)
}

// Sort some unexposed collection of length n by way of functions less and swap.
// less returns whether the element with index i should sort
// before the element with index j.
// swap swaps the elements with indexes i and j.
func Sort3(n int, less func(i, j int) bool, swap func(i, j int)) {
	sd := &sortDelegate{n: n, less: less, swap: swap}
	sort.Sort(sd)
}
