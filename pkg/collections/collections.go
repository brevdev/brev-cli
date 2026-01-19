package collections

import (
	"context"
	"net/http"
	"sort"
	"time"

	"github.com/brevdev/brev-cli/pkg/errors"
	"github.com/jinzhu/copier"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func Duplicate[T any](x T) []T {
	return []T{x, x}
}

// getWithContext creates and sends an HTTP GET request with the provided context
func GetRequestWithContext(ctx context.Context, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, errors.WrapAndTrace(err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, errors.WrapAndTrace(err)
	}
	return resp, nil
}

func Fmap[T, R any](fn func(some T) R, list []T) []R {
	return Foldl(func(acc []R, next T) []R {
		return append(acc, fn(next))
	}, []R{}, list)
}

func Compose[T, S, R any](fn1 func(some S) R, fn2 func(some T) S) func(some T) R {
	return func(some T) R {
		return fn1(fn2(some))
	}
}

func C5[T, S, R, U, V, W any](fn02 func(some V) W, fn01 func(some U) V, fn0 func(some R) U, fn1 func(some S) R, fn2 func(some T) S) func(some T) W {
	return func(some T) W {
		return fn02(fn01(fn0(fn1(fn2(some)))))
	}
}

func ID[T any](x T) T {
	return x
}

func S[T any](fns ...func(some T) T) func(some T) T {
	return Foldl(Compose[T, T, T], ID[T], fns)
}

func P2[X, Y, Z any](fn func(X, Y) Z, x X) func(Y) Z {
	return func(y Y) Z {
		return fn(x, y)
	}
}

func Flip[X, Y, Z any](fn func(X, Y) Z) func(Y, X) Z {
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

func Fanout[T, R any](fs []func(T) R, item T) []R {
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

func Difference[T comparable](from, remove []T) []T {
	returnval := Foldl(func(acc maplist[T], el T) maplist[T] {
		if _, ok := acc.Map[el]; !ok {
			acc.Map[el] = true
			acc.List = append(acc.List, el)
		}
		return acc
	}, maplist[T]{Map: ToDict(remove), List: []T{}}, from)
	return returnval.List
}

func DictMerge[K comparable, V any](left, right map[K]V) map[K]V {
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

func Foldl[T any, R any](fn func(acc R, next T) R, base R, list []T) R {
	for _, value := range list {
		base = fn(base, value)
	}
	return base
}

// loops over list and returns when has returns true
func ListHas[K any](list []K, has func(l K) bool) bool {
	k := Find(list, has)
	// Simplify nil check by returning the expression directly
	return k != nil
}

func ListContains[K comparable](list []K, item K) bool {
	return ListHas(list, func(l K) bool { return l == item })
}

// Take a list of things and a function that returns a list of things then combines list after mapping
// func T -> [R, R, R ...]
// [T, T, T ...] -> [R, R, R ...]
func Flatmap[T, R any](fn func(some T) []R, list []T) []R {
	return Foldl(func(acc []R, el T) []R {
		return Concat(acc, fn(el))
	}, []R{}, list)
}

func Concat[T any](left, right []T) []T {
	return append(left, right...)
}

var timeToPBTimeStamp CopyConverter[time.Time, *timestamppb.Timestamp] = func(src time.Time) (*timestamppb.Timestamp, error) {
	return timestamppb.New(src), nil
}

var timePtrToPBTimeStamp CopyConverter[*time.Time, *timestamppb.Timestamp] = func(src *time.Time) (*timestamppb.Timestamp, error) {
	if src == nil {
		return nil, nil
	}
	return timestamppb.New(*src), nil
}

var pbTimeStampToTime CopyConverter[*timestamppb.Timestamp, time.Time] = func(src *timestamppb.Timestamp) (time.Time, error) {
	return src.AsTime(), nil
}

var pbTimeStampToTimePtr CopyConverter[*timestamppb.Timestamp, *time.Time] = func(src *timestamppb.Timestamp) (*time.Time, error) {
	if src == nil {
		return nil, nil
	}
	t := src.AsTime()
	return &t, nil
}

var DefaultConverters = CopierConverters{
	timeToPBTimeStamp,
	timePtrToPBTimeStamp,
	pbTimeStampToTime,
	pbTimeStampToTimePtr,
}

type CopierConverters []CopierConverter

type CopierConverter interface {
	ToCopierTypeConverter() copier.TypeConverter
}

type CopyConverter[T, R any] func(T) (R, error)

func (c CopyConverter[T, R]) ToCopierTypeConverter() copier.TypeConverter {
	var t T
	var r R
	ctc := copier.TypeConverter{
		SrcType: t,
		DstType: r,
		Fn: func(src interface{}) (dst interface{}, err error) {
			tt, ok := src.(T)
			if !ok {
				return nil, errors.Errorf("cannot convert %T to %T", src, dst)
			}
			return c(tt)
		},
	}
	return ctc
}

type Params[T any] struct {
	Value T
	Ctx   context.Context
}

type Result[T any] struct {
	Value T
	Err   error
}

type MapKeyVal[K comparable, V any] struct {
	Key   K
	Value V
}

// sortFn if i < j then ascending (1,2,3), if i > j then descending (3,2,1)
func SortBy[T any](sortFn func(T, T) bool, list []T) []T {
	sort.SliceStable(list, func(i, j int) bool {
		return sortFn(list[i], list[j])
	})
	return list
}

func Find[T any](list []T, f func(T) bool) *T {
	for _, item := range list {
		if f(item) {
			return &item
		}
	}
	return nil
}

// returns those that are true
func Filter[T any](list []T, f func(T) bool) []T {
	result := []T{}
	for _, item := range list {
		if f(item) {
			result = append(result, item)
		}
	}
	return result
}

type Runnable interface {
	Run(ctx context.Context) error
	Shutdown(ctx context.Context) error
}

type ContextKey string

const IdempotencyKeyName ContextKey = "idempotencyKey"

type AsyncResult[T any] struct {
	result chan result[T]
}

type result[T any] struct {
	value T
	err   error
}

func Async[T any](f func() (T, error)) *AsyncResult[T] {
	r := &AsyncResult[T]{result: make(chan result[T], 1)} // Buffered channel
	go func() {
		value, err := f()
		r.result <- result[T]{value: value, err: err}
	}()
	return r
}

// Await blocks until the asynchronous operation completes, returning the result and error.
func (ar *AsyncResult[T]) Await() (T, error) {
	r := <-ar.result // This will block until the result is available
	return r.value, r.err
}

var (
	// ErrCanceled is the error returned when the context is canceled.
	ErrCanceled = context.Canceled
	// ErrTimeout is the error returned when the context's deadline passes.
	ErrTimeout = context.DeadlineExceeded
)

// DoOption defines the method to customize a DoWithTimeout call.
type DoOption func() context.Context

type TimeoutOptions struct {
	ParentContext context.Context
	CatchPanic    bool
}
