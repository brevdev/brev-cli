package collections

import (
	"context"
	"net/http"
	"sort"
	"sync"
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

func Fmap[T any, R any](fn func(some T) R, list []T) []R {
	return Foldl(func(acc []R, next T) []R {
		return append(acc, fn(next))
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

func C5[T any, S any, R any, U any, V any, W any](fn02 func(some V) W, fn01 func(some U) V, fn0 func(some R) U, fn1 func(some S) R, fn2 func(some T) S) func(some T) W {
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

// Await blocks until the asynchronous operation completes, returning the result and error.

// loops over list and returns when has returns true
func ListHas[K any](list []K, has func(l K) bool) bool {
	k := Find(list, has)
	if k != nil { //nolint:gosimple //ok
		return true
	}
	return false
}

func ListContains[K comparable](list []K, item K) bool {
	return ListHas(list, func(l K) bool { return l == item })
}

// map over a go map

// map over a go map and return a map, merge the maps

func Foldl[T any, R any](fn func(acc R, next T) R, base R, list []T) R {
	for _, value := range list {
		base = fn(base, value)
	}
	return base
}

// Take a list of things and a function that returns a list of things then combines list after mapping (return early from error)
// func T -> [R, R, R ...]
// [T, T, T ...] -> [R, R, R ...]

// Take a list of things and a function that returns a list of things then combines list after mapping
// func T -> [R, R, R ...]
// [T, T, T ...] -> [R, R, R ...]
func Flatmap[T any, R any](fn func(some T) []R, list []T) []R {
	return Foldl(func(acc []R, el T) []R {
		return Concat(acc, fn(el))
	}, []R{}, list)
}

func Concat[T any](left []T, right []T) []T {
	return append(left, right...)
}

// func T -> R
// [T, T, T ...] -> [R, R, R ...]

// return default if ptr is nil or de-referenced ptr value is empty

// right maps override left if they have the same key

// return value or nil if value is zero

//nolint:wrapcheck // fine

type CopyOption interface {
	apply(t any, r any, o *CopyOptions)
}

type CopyMap map[string]string

// For testing different DX

type CopyOptions struct {
	ShallowCopy           bool
	OmitDefaultConverters bool
	Converters            CopierConverters
	Mappers               CopyMappers // create mappings for any arbitrary type
}

type CopyMappers []CopyMapper

type CopyMapper interface {
	ToCopierMapping() copier.FieldNameMapping
}

type DumbCopyMapper struct {
	Mapping copier.FieldNameMapping
}

type CopyMapping[T, R any] map[string]string

func (c CopyMapping[T, R]) ToCopierMapping() copier.FieldNameMapping {
	var t T
	var r R
	return copier.FieldNameMapping{
		SrcType: t,
		DstType: r,
		Mapping: c,
	}
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

// pass in m a map of string to any

type MapKeyVal[K comparable, V any] struct {
	Key   K
	Value V
}

// sortFn if i < j then ascending (1,2,3), if i > j then descending (3,2,1)

// sortFn if i < j then ascending (1,2,3), if i > j then descending (3,2,1)
func SortBy[T any](sortFn func(T, T) bool, list []T) []T {
	sort.SliceStable(list, func(i, j int) bool {
		return sortFn(list[i], list[j])
	})
	return list
}

// takes a list of items and checks if items are elements in another list

func Find[T any](list []T, f func(T) bool) *T {
	for _, item := range list {
		if f(item) {
			return &item
		}
	}
	return nil
}

// returns those that are true

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

// creates a map of the keys in a and not in b

//nolint:gosimple //ok

// Create a buffered channel to act as a semaphore.

// Acquire a token from the semaphore.

// attach call stack to avoid missing in different goroutine

// Release a token back to the semaphore.

//nolint:wrapcheck // fine for internal

// Early returns if one error is found, will return partial work

// Create a buffered channel to act as a semaphore.

// Priority is given to firstErr if it's set

// If firstErr is nil, then we check if the context was canceled

// Assumes that if cont is false res is empty

type Runnable interface {
	Run(ctx context.Context) error
	Shutdown(ctx context.Context) error
}

// RunAllWithShutdown runs Runnabls in parallel and waits for shutdown signal (max n seconds)
// if one runner errors or panics

// attach call stack to avoid missing in different goroutine

// Received shutdown signal

// One of the Run methods returned an error

// Initiate shutdown of all runners

type ContextKey string

const IdempotencyKeyName ContextKey = "idempotencyKey"

// do not check for error because if fail, then just leave as empty string

// for testing, and printing to screen, ignores error

type SafeSlice[T any] struct {
	slice []T
	mu    sync.RWMutex
}

// Append adds a new element to the slice.
func (s *SafeSlice[T]) Append(value ...T) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.slice = append(s.slice, value...)
}

// Get retrieves an element at a specific index.
func (s *SafeSlice[T]) Get(index int) (T, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if index < 0 || index >= len(s.slice) {
		var zero T // Create a zero value of type T
		return zero, false
	}
	return s.slice[index], true
}

func (s *SafeSlice[T]) Set(slice []T) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.slice = slice
}

func (s *SafeSlice[T]) SetAt(index int, value T) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if index < 0 || index >= len(s.slice) {
		return false
	}
	s.slice[index] = value
	return true
}

func (s *SafeSlice[T]) Delete(index int) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if index < 0 || index >= len(s.slice) {
		return false
	}
	s.slice = append(s.slice[:index], s.slice[index+1:]...)
	return true
}

func (s *SafeSlice[T]) Slice() []T {
	s.mu.RLock()
	defer s.mu.RUnlock()
	newSlice := make([]T, len(s.slice))
	for i, v := range s.slice { //nolint:gosimple //ok
		newSlice[i] = v
	}
	return newSlice
}

type SafeCounter struct {
	mu sync.Mutex
	c  int
}

// SafeValue holds an arbitrary value with read and write protection.
// T is the type of the value.
type SafeValue[T any] struct {
	value T
	mutex sync.RWMutex
}

// NewSafeValue creates a new SafeValue.

// Get returns the value safely.
func (sv *SafeValue[T]) Get() T {
	sv.mutex.RLock()
	defer sv.mutex.RUnlock()
	return sv.value
}

// Set updates the value safely.
func (sv *SafeValue[T]) Set(newValue T) {
	sv.mutex.Lock()
	defer sv.mutex.Unlock()
	sv.value = newValue
}

// findStructField looks for a field in the given struct.
// The field being looked for should be a pointer to the actual struct field.
// If found, the field info will be returned. Otherwise, nil will be returned.

// do additional type comparison because it's possible that the address of
// an embedded struct is the same as the first field of the embedded struct

// delve into anonymous struct to look for the field

// returns json string or empty if fails

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

type Rollback struct {
	undos []func() error
}

//nolint:wrapcheck // fine for internal

// SleepWithHealthCheck sleeps for the specified duration `d` and periodically calls `heartbeatFn`
// at every `tickRate` until `d` has elapsed.

// Call the heartbeat function immediately
// Timer to manage the total sleep duration

// Ticker to manage the heartbeat function calls

// Ensures the ticker is stopped to free resources

// On every tick, call the heartbeat function

// Once the total duration has passed, return

// Wait for the sleep duration to pass before returning

// cancel context to end

// cancel context to end

// end early if err

// Mark this function as a helper

// If we reach here, all retries failed

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

// if you loop forever, make sure you have a way to break the loop
// see Test_DoWithTimeoutTimeoutLoop

// if you loop forever, make sure you have a way to break the loop
// see Test_DoWithTimeoutTimeoutLoop

// create channel with buffer size 1 to avoid goroutine leak

// attach call stack to avoid missing in different goroutine

//nolint:wrapcheck // no need to wrap

// WithContext customizes a DoWithTimeout call with given ctx.
