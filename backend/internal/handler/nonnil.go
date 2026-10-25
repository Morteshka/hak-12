package handler

// nonNil превращает пустой (nil) список в [] — иначе JSON получится «null», и сайт падает на .map/.filter.
func nonNil[T any](s []T) []T {
	if s == nil {
		return []T{}
	}
	return s
}
