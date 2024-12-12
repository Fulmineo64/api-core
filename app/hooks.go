package app

const AfterUpdateHook = "AfterUpdate"

type Hook[T any] struct {
	index *int
	Names []string
	Funcs []T
}

func (h *Hook[T]) Before(name string) *Hook[T] {
	for i, n := range h.Names {
		if n == name {
			h.index = &i
			return h
		}
	}
	h.index = nil
	return h
}

func (h *Hook[T]) After(name string) *Hook[T] {
	for i, n := range h.Names {
		if n == name {
			j := i + 1
			h.index = &j
			return h
		}
	}
	h.index = nil
	return h
}

func (h *Hook[T]) Index() int {
	if h.index == nil {
		return len(h.Names)
	} else {
		index := *h.index
		h.index = nil
		return index
	}
}

func (h *Hook[T]) Add(name string, hook T) *Hook[T] {
	index := h.Index()
	if len(h.Funcs) <= index {
		h.Names = append(h.Names, name)
		h.Funcs = append(h.Funcs, hook)
		return h
	}
	h.Names = append(h.Names[:index+1], h.Names[index:]...)
	h.Funcs = append(h.Funcs[:index+1], h.Funcs[index:]...)
	h.Names[index] = name
	h.Funcs[index] = hook
	return h
}

func (h *Hook[T]) Remove(name string) *Hook[T] {
	for i, n := range h.Names {
		if n == name {
			h.Names = append(h.Names[:i], h.Names[i+1:]...)
			h.Funcs = append(h.Funcs[:i], h.Funcs[i+1:]...)
			return h
		}
	}
	return h
}
