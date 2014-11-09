package geom

import (
	"math"
)

type Transform2D interface {
	TransformPoint(pt Point) Point
}

// I don't know what the "right" convention is for storing the
// matrix elements, so I'm using it as follows because it should
// make multiplication of the transform and a vector easier:
//    [ 0 1 2
//      3 4 5
//      6 7 8 ]
type AffineTransform2D [9]float64

func (t *AffineTransform2D) TransformPoint(pt Point) Point {
	x := pt.X*t[0] + pt.Y*t[1] + t[2]
	y := pt.X*t[3] + pt.Y*t[4] + t[5]
	// We assume the transform is affine, and thus the 3rd row is 0,0,1.
	return Point{x, y}
}

// Matrix multiplication (for combining transformations.
//    [ 0 1 2     [ a b c
//      3 4 5   *   d e f
//      6 7 8 ]     g h k ]
func (X *AffineTransform2D) MultiplyTransforms(
	Y *AffineTransform2D) (Z *AffineTransform2D) {
	Z = new(AffineTransform2D)
	for xr := 0; xr < 3; xr++ {
		xi := xr * 3
		for yc := 0; yc < 3; yc++ {
			Z[xi+yc] = X[xi]*Y[yc] + X[xi+1]*Y[yc+3] + X[xi+2]*Y[yc+6]
		}
	}
	return
}

func NewRotateTransform2D(theta float64) *AffineTransform2D {
	c := math.Cos(theta)
	s := math.Sin(theta)
	return &AffineTransform2D{
		c, s, 0,
		-s, c, 0,
		0, 0, 1,
	}
}

func NewTranslateTransform2D(dx, dy float64) *AffineTransform2D {
	return &AffineTransform2D{
		1, 0, dx,
		0, 1, dy,
		0, 0, 1,
	}
}

// See http://en.wikipedia.org/wiki/Invertible_matrix#Inversion_of_3.C3.973_matrices
//
// X = [a b c                        1    [A D G
//      d e f         Y = inv(X) = ------  B E H
//      g h k]                     det(X)  C F K]
//
// (We use k, not i, after h so as not to confuse the element with the
// identity matrix, identified with I.)
func (X *AffineTransform2D) Invert() *AffineTransform2D {
	dh := X[3] * X[7]
	dk := X[3] * X[8]
	eg := X[4] * X[6]
	ek := X[4] * X[8]
	fg := X[5] * X[6]
	fh := X[5] * X[7]

	// det(X) = a(ek-fh)-b(kd-fg)+c(dh-eg).
	detX := X[0]*(ek-fh) - X[1]*(dk-fg) + X[2]*(dh-eg)

	ae := X[0] * X[4]
	af := X[0] * X[5]
	ah := X[0] * X[7]
	ak := X[0] * X[8]
	bd := X[1] * X[3]
	bf := X[1] * X[5]
	bg := X[1] * X[6]
	bk := X[1] * X[8]
	cd := X[2] * X[3]
	ce := X[2] * X[4]
	cg := X[2] * X[6]
	ch := X[2] * X[7]

	A := ek - fh
	B := -(dk - fg)
	C := dh - eg
	D := -(bk - ch)
	E := ak - cg
	F := -(ah - bg)
	G := bf - ce
	H := -(af - cd)
	K := ae - bd

	Y := &AffineTransform2D{
		A, D, G,
		B, E, H,
		C, F, K,
	}

	return Y.ScalarMultiple(1 / detX)
}

func (p *AffineTransform2D) ScalarMultiple(v float64) *AffineTransform2D {
	n := new(AffineTransform2D)
	for i := range p {
		n[i] = p[i] * v
	}
	return n
}
