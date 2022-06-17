//go:build !codeanalysis

package collections

import (
	"sort"
)

func Duplicate[T any](x T) []T {
	return []T{x, x}
}

func Foldl[T any, R any](fn func(acc R, next T) R, base R, list []T) R {
	for _, value := range list {
		base = fn(base, value)
	}

	return base
}

func Foldr[T any, R any](fn func(next T, carry R) R, base R, list []T) R {
	for idx := len(list) - 1; idx >= 0; idx-- {
		base = fn(list[idx], base)
	}

	return base
}

func Concat[T any](left []T, right []T) []T {
	return Foldl(func(acc []T, next T) []T {
		return append(acc, next)
	}, left, right)
}

func Fmap[T any, R any](fn func(some T) R, list []T) []R {
	return Foldl(func(acc []R, next T) []R {
		return append(acc, fn(next))
	}, []R{}, list)
}

func Filter[T any](fn func(some T) bool, list []T) []T {
	return Foldl(func(acc []T, next T) []T {
		if fn(next) {
			acc = append(acc, next)
		}
		return acc
	}, []T{}, list)
}

func Flatmap[T any, R any](fn func(some T) []R, list []T) []R {
	return Foldl(func(acc []R, el T) []R {
		return Concat(acc, fn(el))
	}, []R{}, list)
}

// there is no function overloading [and the need to describe dependent relations between the types of the functions rules out variadic arguments]
// so we will define c2, c3, c4, and c5 which will allow simple composition of up to 5 functions
// anything more than that should be refactored so that subcomponents of the composition are renamed, anyway (or named itself)

func Compose[T any, S any, R any](fn1 func(some S) R, fn2 func(some T) S) func(some T) R {
	return func(some T) R {
		return fn1(fn2(some))
	}
}

func C2[T any, S any, R any](fn1 func(some S) R, fn2 func(some T) S) func(some T) R {
	return Compose(fn1, fn2)
}

func C3[T any, S any, R any, U any](fn0 func(some R) U, fn1 func(some S) R, fn2 func(some T) S) func(some T) U {
	return func(some T) U {
		return fn0(fn1(fn2(some)))
	}
}

func C4[T any, S any, R any, U any, V any](fn01 func(some U) V, fn0 func(some R) U, fn1 func(some S) R, fn2 func(some T) S) func(some T) V {
	return func(some T) V {
		return fn01(fn0(fn1(fn2(some))))
	}
}

func C5[T any, S any, R any, U any, V any, W any](fn02 func(some V) W, fn01 func(some U) V, fn0 func(some R) U, fn1 func(some S) R, fn2 func(some T) S) func(some T) W {
	return func(some T) W {
		return fn02(fn01(fn0(fn1(fn2(some)))))
	}
}

func Id[T any](x T) T {
	return x
}

func C[T any](fns ...func(some T) T) func(some T) T {
	return Foldr(Compose[T, T, T], Id[T], fns)
}

func S[T any](fns ...func(some T) T) func(some T) T {
	return Foldl(Compose[T, T, T], Id[T], fns)
}

func P2[X any, Y any, Z any](fn func(X, Y) Z, x X) func(Y) Z {
	return func(y Y) Z {
		return fn(x, y)
	}
}

func Flip[X any, Y any, Z any](fn func(X, Y) Z) func(Y, X) Z {
	return func(y Y, x X) Z {
		return fn(x, y)
	}
}

func First[X any](list []X) *X {
	if len(list) > 0 {
		return &list[0]
	}
	return nil
}

func SortBy[X any](sortFn func(X, X) bool, list []X) []X {
	// sort.sliceStable(list, sortFn)
	sort.SliceStable(list, func(i, j int) bool {
		return sortFn(list[i], list[j])
	})
	return list

	// if it's empty, it's sorted
	// if it has one element, it's sorted
	// take the first element as the pivot
	// partition the rest of the list as to whether it is greater than that element or not
	// enconcat the sortBys of both of those lists with the pivot element
	// if len(list) < 2 {
	// 	return list
	// }
	// pivot := *First(list)

	// return Enconcat(SortBy(sortFn, before), pivot, sortBy(sortFn, after))
}

func Cons[X any](x X, list []X) []X {
	return Concat([]X{x}, list)
}

func Enconcat[X any](before []X, x X, after []X) []X {
	return Concat(before, Cons(x, after))
}

func Any[T any](f func(T) bool, list []T) {
	Foldl(func(acc bool, el T) bool {
		if acc {
			return acc
		} else {
			return f(el)
		}
	}, false, list)
}

func Fanout[T any, R any](fs []func(T) R, item T) []R {
	return Fmap(func(f func(T) R) R {
		return f(item)
	}, fs)
}

type maplist[T comparable] struct {
	List []T
	Map  map[T]bool
}

func Uniq[T comparable](xs []T) []T {
	result := Foldl(func(acc maplist[T], el T) maplist[T] {
		if _, ok := acc.Map[el]; !ok {
			acc.Map[el] = true
			acc.List = append(acc.List, el)
		}
		return acc
	}, maplist[T]{List: []T{}, Map: map[T]bool{}}, xs)
	return result.List
}

func ToDict[T comparable](xs []T) map[T]bool {
	return Foldl(func(acc map[T]bool, el T) map[T]bool {
		acc[el] = true
		return acc
	}, map[T]bool{}, xs)
}

func Difference[T comparable](from []T, remove []T) []T {
	returnval := Foldl(func(acc maplist[T], el T) maplist[T] {
		if _, ok := acc.Map[el]; !ok {
			acc.Map[el] = true
			acc.List = append(acc.List, el)
		}
		return acc
	}, maplist[T]{Map: ToDict(remove), List: []T{}}, from)
	return returnval.List
}

func DictMerge[K comparable, V any](left map[K]V, right map[K]V) map[K]V {
	newMap := map[K]V{}
	for key, val := range left {
		if _, ok := right[key]; ok {
			newMap[key] = right[key]
		} else {
			newMap[key] = val
		}
	}
	return newMap
}

func Keys[T comparable, R any](dict map[T]R) []T {
	keys := make([]T, len(dict))
	i := 0
	for k := range dict {
		keys[i] = k
		i++
	}
	return keys
}

func FilterEmpty[T comparable](l []T) []T {
	var zero T
	out := []T{}
	for _, i := range l {
		if i != zero {
			out = append(out, i)
		}
	}
	return out
}

func Contains[T comparable](s []T, e T) bool {
	for _, v := range s {
		if v == e {
			return true
		}
	}
	return false
}
