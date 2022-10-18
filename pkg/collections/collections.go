//go:build !codeanalysis

package collections

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"time"

	"github.com/brevdev/brev-cli/pkg/errors"
	"github.com/jinzhu/copier"
	"github.com/mitchellh/mapstructure"
	"golang.org/x/exp/constraints"
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

func MapContainsKey[K comparable, V any](m map[K]V, key K) bool {
	_, ok := m[key]
	return ok
}

func ValueOrZero[T any](ptr *T) T {
	if ptr == nil {
		res, _ := reflect.Zero(reflect.TypeOf(ptr)).Interface().(T)
		return res
	} else {
		return *ptr
	}
}

func ListHas[K any](list []K, item K, has func(l, r K) bool) bool {
	for _, listItem := range list {
		if has(listItem, item) {
			return true
		}
	}
	return false
}

func ListContains[K comparable](list []K, item K) bool {
	return ListHas(list, item, func(l, r K) bool { return l == r })
}

func ManyIntegersToInts[T constraints.Integer](i []T) []int {
	return Map(i, func(i T) int { return int(i) })
}

func ManyStringLikeToStrings[T ~string](i []T) []string {
	return Map(i, func(i T) string { return string(i) })
}

func MapE[T, R any](items []T, mapper func(T) (R, error)) ([]R, error) {
	results := []R{}
	for _, item := range items {
		res, err := mapper(item)
		if err != nil {
			return results, errors.WrapAndTrace(err)
		}
		results = append(results, res)
	}
	return results, nil
}

func Accumulate[T any](items []T, accumulator func(T, T) T) T {
	result := items[0]
	for _, item := range items[1:] {
		result = accumulator(result, item)
	}
	return result
}

func AccumulateE[T any](items []T, accumulator func(T, T) (T, error)) (T, error) {
	result := items[0]
	for _, item := range items[1:] {
		var err error
		result, err = accumulator(result, item)
		if err != nil {
			var zero T
			return zero, errors.WrapAndTrace(err)
		}
	}
	return result, nil
}

func Flatten[T any](listOfLists [][]T) []T {
	result := []T{}
	for _, list := range listOfLists {
		result = append(result, list...)
	}
	return result
}

func FoldlE[T any, R any](fn func(acc R, next T) (R, error), base R, list []T) (R, error) {
	for _, value := range list {
		var err error
		base, err = fn(base, value)
		if err != nil {
			var zero R
			return zero, errors.WrapAndTrace(err)
		}
	}
	return base, nil
}

// Take a list of things and a function that returns a list of things then combines list after mapping (return early from error)
// func T -> [R, R, R ...]
// [T, T, T ...] -> [R, R, R ...]
func FlatmapE[T any, R any](fn func(some T) ([]R, error), list []T) ([]R, error) {
	return FoldlE(func(acc []R, el T) ([]R, error) {
		res, err := fn(el)
		if err != nil {
			return nil, errors.WrapAndTrace(err)
		}
		return Concat(acc, res), nil
	}, []R{}, list)
}

// func T -> R
// [T, T, T ...] -> [R, R, R ...]
func Map[T, R any](items []T, mapper func(T) R) []R {
	results := []R{}
	for _, item := range items {
		results = append(results, mapper(item))
	}
	return results
}

func MapFromList[T any, R comparable](list []T, keySelector func(l T) R) map[R]T {
	result := map[R]T{}
	for _, item := range list {
		result[keySelector(item)] = item
	}
	return result
}

func ListOfPointersToListOfValues[T any](list []*T) []T {
	return Map(list, func(i *T) T { return *i })
}

func DefaultValue[T any](value T, defaultValue T) T {
	if reflect.ValueOf(value).IsZero() {
		return defaultValue
	} else {
		return value
	}
}

func DefaultPtr[T any](value *T, defaultValue T) T {
	if reflect.ValueOf(value).IsZero() {
		return defaultValue
	} else {
		return *value
	}
}

// return default if ptr is nil or de-referenced ptr value is empty
func DefaultPtrOrValue[T any](value *T, defaultValue T) T {
	if value == nil || reflect.ValueOf(*value).IsZero() {
		return defaultValue
	} else {
		return *value
	}
}

// right maps override left if they have the same key
func MergeMaps[K comparable, V any](maps ...map[K]V) map[K]V {
	result := map[K]V{}
	for _, m := range maps {
		for k, v := range m {
			result[k] = v
		}
	}
	return result
}

func GetAValueFromMap[K comparable, V any](m map[K]V) *V {
	for _, v := range m {
		return &v
	}
	return nil
}

func ListOfSomethingToListOfAny[T any](l []T) []any {
	newList := []any{}
	for _, item := range l {
		newList = append(newList, item)
	}
	return newList
}

func Ptr[T any](x T) *T {
	return &x
}

func Deref[T any](x *T) T {
	return *x
}

// return value or nil if value is zero
func ZeroValueToNil[T any](x T) *T {
	if reflect.ValueOf(x).IsZero() {
		return nil
	} else {
		return &x
	}
}

func IsEmpty[T any](x T) bool {
	return reflect.ValueOf(x).IsZero()
}

func IsEmptyValP[T any](x *T) bool {
	if x == nil {
		return true
	} else {
		return reflect.ValueOf(*x).IsZero()
	}
}

func ReturnOnCondition[T any](ctx context.Context, fetcher func(ctx context.Context) (T, error), conditional func(i T) bool, updateDuration time.Duration) (T, error) {
	for {
		result, err := fetcher(ctx)
		if err != nil {
			return result, errors.WrapAndTrace(err)
		}
		if conditional(result) {
			return result, nil
		}
		time.Sleep(updateDuration)
	}
}

func InitialNotEqual[T any](i T) func(l T) bool {
	return func(l T) bool {
		return !reflect.DeepEqual(i, l)
	}
}

func FromJSON[T any](j []byte) (T, error) {
	var t T
	err := json.Unmarshal(j, &t)
	if err != nil {
		return t, errors.WrapAndTrace(err)
	}
	return t, nil
}

func TryCopyToNew[T any, R any](t T, options copier.Option) (R, error) {
	var r R
	if err := copier.CopyWithOption(&r, t, options); err != nil {
		return r, errors.WrapAndTrace(err)
	}
	return r, nil
}

func TryCopyTo[T any, R any](t T, r R, options copier.Option) (R, error) {
	if err := copier.CopyWithOption(&r, t, options); err != nil {
		return r, errors.WrapAndTrace(err)
	}
	return r, nil
}

func GetMapKeys[K comparable, V any](m map[K]V) []K {
	keys := []K{}
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func GetMapValues[K comparable, V any](m map[K]V) []V {
	values := []V{}
	for _, v := range m {
		values = append(values, v)
	}
	return values
}

type Params[T any] struct {
	Value T
	Ctx   context.Context
}

type Result[T any] struct {
	Value T
	Err   error
}

func (r Result[T]) Unwrap() (T, error) {
	return r.Value, r.Err
}

// pass in m a map of string to any
func MapToStruct[T any](m any) (T, error) {
	var t T
	err := mapstructure.Decode(m, &t)
	if err != nil {
		return t, errors.WrapAndTrace(err)
	}
	return t, nil
}

func RemoveDuplicates[T comparable](list []T) []T {
	seen := map[T]bool{}
	result := []T{}
	for _, item := range list {
		if !seen[item] {
			result = append(result, item)
			seen[item] = true
		}
	}
	return result
}

func ContainsDuplicatesErr[T comparable](list []T) error {
	seen := map[T]bool{}
	for _, item := range list {
		if seen[item] {
			return fmt.Errorf("duplicate item: %v", item)
		}
		seen[item] = true
	}
	return nil
}

// takes a list of items and checks if items are elements in another list
func ListItemsAreErr[T comparable](items []T, are []T) error {
	check := map[T]bool{}
	for _, r := range are {
		check[r] = true
	}
	for _, i := range items {
		if !check[i] {
			return fmt.Errorf("item %v is not in list %v", i, are)
		}
	}
	return nil
}

func Find[T any](list []*T, f func(*T) bool) *T {
	for _, item := range list {
		if f(item) {
			return item
		}
	}
	return nil
}
