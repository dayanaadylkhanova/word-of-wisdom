package quote

import (
	mrand "math/rand/v2"
)

type Static struct {
	list []string
	r    *mrand.Rand
}

func NewStatic() *Static {
	return &Static{
		list: []string{
			"“Do. Or do not. There is no try.” – Yoda",
			"“Simplicity is the soul of efficiency.” – Austin Freeman",
			"“Programs must be written for people to read.” – Harold Abelson",
			"“Premature optimization is the root of all evil.” – Donald Knuth",
			"“Talk is cheap. Show me the code.” – Linus Torvalds",
		},
	}
}

// NewStaticWith доп. конструктор для тестов/DI
func NewStaticWith(list []string, r *mrand.Rand) *Static {
	return &Static{list: list, r: r}
}

func (s *Static) Random() string {
	if len(s.list) == 0 {
		return "" // защита от panic
	}
	if s.r != nil {
		return s.list[s.r.IntN(len(s.list))]
	}
	return s.list[mrand.IntN(len(s.list))] // как раньше
}
