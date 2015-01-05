package util

import (
	"bytes"
	"fmt"
	"strings"
)

func JoinErrors(errors []error, sep string) string {
	a := make([]string, len(errors))
	for i := range errors {
		a[i] = errors[i].Error()
	}
	return strings.Join(a, sep)
}

type Errors interface {
	// Add an error (if non-nil).
	AddError(err error)

	NumErrors() int

	// TODO Use these to provide more detail about what is going on when
	// an error occurs (e.g. what the callers were trying to have accomplished).
	PushContext(ctx string)
	PopContext(ctx string) int

	ToError() error

	Error() string
}

// TODO Maybe add (context... string) arg, so that caller can provide context
// for errors via nested errors with context. Then when AddError is called,
// if it is an Errors instance, we can examine it to see if any errors during
// the context.
func NewErrors() Errors {
	return &errorsImpl{}
}

type context struct {
	ctx        string
	start, end int
}

type errorsImpl struct {
	errors       []error
	contextStack []*context
	contexts     []*context
}

func (p *errorsImpl) AddError(err error) {
	if err != nil {
		p.errors = append(p.errors, err)
	}
}

func (p *errorsImpl) NumErrors() int {
	return len(p.errors)
}

func (p *errorsImpl) PushContext(ctx string) {
	c := &context{ctx, len(p.errors), -1}
	p.contextStack = append(p.contextStack, c)
	p.contextStack = append(p.contexts, c)
}

func (p *errorsImpl) PopContext(ctx string) int {
	if len(p.contextStack) == 0 {
		panic(fmt.Errorf(`Can't pop context "%s" off of an empty stack`, ctx))
	}
	lastIndex := len(p.contextStack) - 1
	c := p.contextStack[lastIndex]
	if c.ctx != ctx {
		panic(fmt.Errorf(
			`Can't pop context "%s" off stack, doesn't match top context "%s"`,
			ctx, c.ctx))
	}
	c.end = p.NumErrors()
	p.contextStack = p.contextStack[:lastIndex]
	return c.end - c.start
}

func (p *errorsImpl) ToError() error {
	if p.NumErrors() == 0 {
		return nil
	} else {
		return p
	}
}

func (p *errorsImpl) ToString() string {
	if p.NumErrors() == 0 {
		return ""
	}
	// TODO Include context in which errors occurred.

	if p.NumErrors() == 1 {
		return p.errors[0].Error()
	}

	var b bytes.Buffer
	fmt.Fprintf(&b, "%d Errors:", p.NumErrors())
	for n, err := range p.errors {
		fmt.Fprintf(&b, "\n[%d]: %s", n, err)
	}
	return b.String()
}

func (p *errorsImpl) Error() string {
	// TODO Should I treat zero errors as an error here?
	return p.ToString()
}
