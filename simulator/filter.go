package main

type datapoint interface {
	~int | ~float32 | ~float64 | ~int32 | ~int64 | ~uint32 | ~uint64
}

type maximum[T datapoint] struct {
	max T
}

func (m *maximum[T]) record(val T) {
	if val > m.max {
		m.max = val
	}
	return
}

func (m *maximum[T]) get() T {
	res := m.max
	m.max = 0
	return res
}

func (m *maximum[T]) reset() {
	m.max = 0
}

type difference[T datapoint] struct {
	lastRead T
	current  T
}

func (d *difference[T]) record(val T) {
	d.current = val
}

func (d *difference[T]) get() T {
	diff := d.current - d.lastRead
	d.lastRead = d.current
	return diff
}

func (d *difference[T]) reset() {
	d.lastRead = 0
	d.current = 0
}
