package mapper

// Mapper provides type-safe, nil-safe conversion between two types.
type Mapper[A, B any] struct {
	forward  func(*A) *B
	backward func(*B) *A
}

// NewMapper creates a bidirectional Mapper (A->B and B->A).
func NewMapper[A, B any](aToB func(*A) *B, bToA func(*B) *A) *Mapper[A, B] {
	return &Mapper[A, B]{forward: aToB, backward: bToA}
}

// NewForwardMapper creates a unidirectional Mapper (A->B only).
// Calling Reverse/ReverseSlice on a forward-only Mapper returns nil.
func NewForwardMapper[A, B any](aToB func(*A) *B) *Mapper[A, B] {
	return &Mapper[A, B]{forward: aToB}
}

// Map converts A to B. Returns nil when input is nil.
func (m *Mapper[A, B]) Map(a *A) *B {
	if a == nil || m.forward == nil {
		return nil
	}
	return m.forward(a)
}

// Reverse converts B to A. Returns nil when input is nil or Mapper is forward-only.
func (m *Mapper[A, B]) Reverse(b *B) *A {
	if b == nil || m.backward == nil {
		return nil
	}
	return m.backward(b)
}

// MapSlice batch-converts []*A to []*B, skipping nil entries.
func (m *Mapper[A, B]) MapSlice(as []*A) []*B {
	if len(as) == 0 {
		return nil
	}
	bs := make([]*B, 0, len(as))
	for _, a := range as {
		if b := m.Map(a); b != nil {
			bs = append(bs, b)
		}
	}
	return bs
}

// ReverseSlice batch-converts []*B to []*A, skipping nil entries.
func (m *Mapper[A, B]) ReverseSlice(bs []*B) []*A {
	if len(bs) == 0 {
		return nil
	}
	as := make([]*A, 0, len(bs))
	for _, b := range bs {
		if a := m.Reverse(b); a != nil {
			as = append(as, a)
		}
	}
	return as
}
