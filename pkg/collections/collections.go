package collections

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"reflect"
	"runtime"
	"runtime/debug"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/brevdev/brev-cli/pkg/errors"
	"github.com/cenkalti/backoff/v4"
	"github.com/jinzhu/copier"
	"github.com/mitchellh/mapstructure"
	"golang.org/x/exp/constraints"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
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

func Foldr[T any, R any](fn func(next T, carry R) R, base R, list []T) R {
	for idx := len(list) - 1; idx >= 0; idx-- {
		base = fn(list[idx], base)
	}

	return base
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

func ID[T any](x T) T {
	return x
}

func C[T any](fns ...func(some T) T) func(some T) T {
	return Foldr(Compose[T, T, T], ID[T], fns)
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

func MapFromList[T any, R comparable](list []T, keySelector func(l T) R) map[R]T {
	result := map[R]T{}
	for _, item := range list {
		result[keySelector(item)] = item
	}
	return result
}

// Await blocks until the asynchronous operation completes, returning the result and error.
func Except[T comparable](items []T, except []T) []T {
	return Filter(items, func(item T) bool {
		return !ListContains(except, item)
	})
}

// loops over list and returns when has returns true
func ListHas[K any](list []K, has func(l K) bool) bool {
	k := Find(list, has)
	if k != nil {
		return true
	}
	return false
}

func MapHasKey[K comparable, V any](m map[K]V, key K) bool {
	_, ok := m[key]
	return ok
}

func ListContains[K comparable](list []K, item K) bool {
	return ListHas(list, func(l K) bool { return l == item })
}

func ManyIntegersToInts[T constraints.Integer](i []T) []int {
	return Map(i, func(i T) int { return int(i) })
}

func ManyStringLikeToStrings[T ~string](i []T) []string {
	return Map(i, func(i T) string { return string(i) })
}

// map over a go map
func MapMap[K comparable, V any, R any](m map[K]V, f func(K, V) R) []R {
	results := []R{}
	for k, v := range m {
		r := f(k, v)
		results = append(results, r)
	}
	return results
}

func MapMapE[K comparable, V any, R any](m map[K]V, f func(K, V) (R, error)) ([]R, error) {
	results := []R{}
	for k, v := range m {
		r, e := f(k, v)
		if e != nil {
			return nil, e
		}
		results = append(results, r)
	}
	return results, nil
}

// map over a go map and return a map, merge the maps
func MapMapMerge[K1 comparable, V1 any, K2 comparable, V2 any](m map[K1]V1, f func(K1, V1) map[K2]V2) map[K2]V2 {
	return MergeMaps(MapMap(m, f)...)
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

func ParallelMapE[T, R any](items []T, mapper func(T) (R, error)) ([]R, error) {
	return ParallelWorkerMapE(items, mapper, 100)
}

func AccumulateMap[A any, K comparable, V any](m map[K]V, accumulator func(A, K, V) A) A {
	var result A
	for k, v := range m {
		result = accumulator(result, k, v)
	}
	return result
}

func Accumulate[A any, T any](items []T, accumulator func(A, T) A) A {
	var result A
	for _, item := range items {
		result = accumulator(result, item)
	}
	return result
}

func AccumulateE[A any, T any](items []T, accumulator func(A, T) (A, error)) (A, error) {
	var result A
	for _, item := range items {
		var err error
		result, err = accumulator(result, item)
		if err != nil {
			var zero A
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

func Foldl[T any, R any](fn func(acc R, next T) R, base R, list []T) R {
	for _, value := range list {
		base = fn(base, value)
	}
	return base
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
func Map[T, R any](items []T, mapper func(T) R) []R {
	results := []R{}
	for _, item := range items {
		results = append(results, mapper(item))
	}
	return results
}

func ListToMap[T any, R comparable](list []T, keySelector func(l T) R) map[R]T {
	result := map[R]T{}
	for _, item := range list {
		result[keySelector(item)] = item
	}
	return result
}

func ListToMapE[T any, R comparable](list []T, keySelector func(l T) (R, error)) (map[R]T, error) {
	result := map[R]T{}
	for _, item := range list {
		res, err := keySelector(item)
		if err != nil {
			return nil, errors.WrapAndTrace(err)
		}
		result[res] = item
	}
	return result, nil
}

func ListToMapKV[T any, R comparable, V any](list []T, keySelector func(l T) R, valueSelector func(l T) V) map[R]V {
	result := map[R]V{}
	for _, item := range list {
		result[keySelector(item)] = valueSelector(item)
	}
	return result
}

func ListToCollisionMap[T comparable](list []T) map[T]bool {
	result := map[T]bool{}
	for _, item := range list {
		result[item] = true
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

func MapToGenericMap[v any](m map[string]v) map[string]interface{} {
	result := map[string]interface{}{}
	for k, v := range m {
		result[k] = v
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
	for ctx.Err() == nil {
		result, err := fetcher(ctx)
		if err != nil {
			return result, errors.WrapAndTrace(err)
		}
		if conditional(result) {
			return result, nil
		}
		time.Sleep(updateDuration)
	}
	var t T
	return t, ctx.Err() //nolint:wrapcheck // fine
}

func InitialNotEqual[T any](i T) func(l T) bool {
	return func(l T) bool {
		return !reflect.DeepEqual(i, l)
	}
}

func DeepEqual[T any](i T, l T) bool {
	return reflect.DeepEqual(i, l)
}

func FromJSON[T any](j []byte) (T, error) {
	var t T
	err := json.Unmarshal(j, &t)
	if err != nil {
		return t, errors.WrapAndTrace(err)
	}
	return t, nil
}

func CopyVal[T any](t T) (T, error) {
	var r T
	if err := copier.CopyWithOption(&r, t, CopyOptions{}.ToCopierOptions()); err != nil {
		return r, errors.WrapAndTrace(err)
	}
	return r, nil
}

func CopyPtrVal[T any](t *T) (*T, error) {
	var r T
	if err := copier.CopyWithOption(&r, t, CopyOptions{}.ToCopierOptions()); err != nil {
		return nil, errors.WrapAndTrace(err)
	}
	return &r, nil
}

func TryCopyToNew[T any, R any](t T, options ...CopyOption) (R, error) {
	var r R
	copyOptions := CopyOptions{}
	for _, o := range options {
		o.apply(t, r, &copyOptions)
	}
	if err := copier.CopyWithOption(&r, t, copyOptions.ToCopierOptions()); err != nil {
		return r, errors.WrapAndTrace(err)
	}
	return r, nil
}

type CopyOption interface {
	apply(t any, r any, o *CopyOptions)
}

type CopyMap map[string]string

func (c CopyMap) apply(t any, r any, o *CopyOptions) {
	o.Mappers = append(o.Mappers, DumbCopyMapper{Mapping: copier.FieldNameMapping{
		SrcType: t,
		DstType: r,
		Mapping: c,
	}})
}

func TryCopyToNewOptions[T any, R any](t T, options CopyOptions) (R, error) {
	var r R
	if err := copier.CopyWithOption(&r, t, options.ToCopierOptions()); err != nil {
		return r, errors.WrapAndTrace(err)
	}
	return r, nil
}

// For testing different DX
func TryCopyToNewE[T any, R any](t T, mappers ...CopyMappingFunc[T, R]) (R, error) {
	var r R
	opts := CopyOptions{}.ToCopierOptions()
	opts.FieldNameMapping = append(opts.FieldNameMapping, Map(mappers, func(m CopyMappingFunc[T, R]) copier.FieldNameMapping { return m.ToCopierMapping() })...)
	if err := copier.CopyWithOption(&r, t, opts); err != nil {
		return r, errors.WrapAndTrace(err)
	}
	return r, nil
}

type CopyOptions struct {
	ShallowCopy           bool
	OmitDefaultConverters bool
	Converters            CopierConverters
	Mappers               CopyMappers // create mappings for any arbitrary type
}

type CopyMappers []CopyMapper

func (c CopyMappers) ToCopierMappings() []copier.FieldNameMapping {
	var result []copier.FieldNameMapping
	for _, cc := range c {
		result = append(result, cc.ToCopierMapping())
	}
	return result
}

type CopyMapper interface {
	ToCopierMapping() copier.FieldNameMapping
}

type DumbCopyMapper struct {
	Mapping copier.FieldNameMapping
}

func (d DumbCopyMapper) ToCopierMapping() copier.FieldNameMapping {
	return d.Mapping
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

type CopyMappingFunc[T, R any] func(T, R) map[any]any

// Doesn't work but leaving for future reference
func (c CopyMappingFunc[T, R]) ToCopierMapping() copier.FieldNameMapping {
	var t T
	valueT := reflect.ValueOf(&t).Elem()
	var r R
	valueR := reflect.ValueOf(&r).Elem()
	copierMapping := map[string]string{}
	realMapping := c(t, r)
	var allErr error
	for tf, rf := range realMapping {
		tfFound := findStructField(valueT, reflect.ValueOf(tf))
		rfFound := findStructField(valueR, reflect.ValueOf(rf))
		isErr := false
		if tfFound == nil {
			allErr = errors.Join(allErr, errors.Errorf("field %s not found in struct %T", tf, t))
			isErr = true
		}
		if rfFound == nil {
			allErr = errors.Join(allErr, errors.Errorf("field %s not found in struct %T", rf, r))
			isErr = true
		}
		if !isErr {
			copierMapping[tfFound.Name] = rfFound.Name
		}
	}
	if allErr != nil {
		panic(allErr)
	}
	return copier.FieldNameMapping{
		SrcType: t,
		DstType: r,
		Mapping: copierMapping,
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

func (c CopyOptions) ToCopierOptions() copier.Option {
	convs := c.Converters.ToCopierTypeConverters()
	if !c.OmitDefaultConverters {
		convs = append(convs, DefaultConverters.ToCopierTypeConverters()...)
	}
	return copier.Option{
		DeepCopy:         !c.ShallowCopy,
		Converters:       convs,
		FieldNameMapping: c.Mappers.ToCopierMappings(),
	}
}

type CopierConverters []CopierConverter

func (c CopierConverters) ToCopierTypeConverters() []copier.TypeConverter {
	var result []copier.TypeConverter
	for _, cc := range c {
		result = append(result, cc.ToCopierTypeConverter())
	}
	return result
}

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

func TryCopyTo[T any, R any](t T, r R) (R, error) {
	if err := copier.CopyWithOption(&r, t, CopyOptions{}.ToCopierOptions()); err != nil {
		return r, errors.WrapAndTrace(err)
	}
	return r, nil
}

func TryCopyToOptions[T any, R any](t T, r R, options CopyOptions) (R, error) {
	if err := copier.CopyWithOption(&r, t, options.ToCopierOptions()); err != nil {
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

type MapKeyVal[K comparable, V any] struct {
	Key   K
	Value V
}

func MapToList[K comparable, T any](m map[K]T) []MapKeyVal[K, T] {
	var results []MapKeyVal[K, T]
	for k, v := range m {
		results = append(results, MapKeyVal[K, T]{Key: k, Value: v})
	}
	return results
}

// sortFn if i < j then ascending (1,2,3), if i > j then descending (3,2,1)
func SortByE[T any](sortFn func(T, T) (bool, error), list []T) ([]T, error) {
	var retErr error
	sort.SliceStable(list, func(i, j int) bool {
		cmp, err := sortFn(list[i], list[j])
		if err != nil {
			retErr = err
		}
		return cmp
	})
	return list, retErr
}

// sortFn if i < j then ascending (1,2,3), if i > j then descending (3,2,1)
func SortBy[T any](sortFn func(T, T) bool, list []T) []T {
	sort.SliceStable(list, func(i, j int) bool {
		return sortFn(list[i], list[j])
	})
	return list
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
			return errors.Errorf("duplicate item: %v", item)
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
			return errors.Errorf("item %v is not in list %v", i, are)
		}
	}
	return nil
}

func Find[T any](list []T, f func(T) bool) *T {
	for _, item := range list {
		if f(item) {
			return &item
		}
	}
	return nil
}

func FindPtr[T any](list []*T, f func(*T) bool) *T {
	for _, item := range list {
		if f(item) {
			return item
		}
	}
	return nil
}

// returns those that are true
func FilterE[T any](list []T, f func(T) (bool, error)) ([]T, error) {
	result := []T{}
	for _, item := range list {
		res, err := f(item)
		if err != nil {
			return nil, errors.WrapAndTrace(err)
		}
		if res {
			result = append(result, item)
		}
	}
	return result, nil
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

func FilterOutEmpty[T any](list []T) []T {
	return Filter(list, func(i T) bool {
		return !IsEmpty(i)
	})
}

func Max[T constraints.Ordered](x T, y T) T {
	if x > y {
		return x
	} else {
		return y
	}
}

func Min[T constraints.Ordered](x T, y T) T {
	if x < y {
		return x
	} else {
		return y
	}
}

func Deduplicate[T comparable](list []T) []T {
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

func MultiGroupBy[T comparable, A any](list []A, f func(A) []T) map[T][]A {
	result := map[T][]A{}
	for _, item := range list {
		key := f(item)
		for _, k := range key {
			result[k] = append(result[k], item)
		}
	}
	return result
}

func GroupBy[K comparable, A any](list []A, keyGetter func(A) K) map[K][]A {
	result := map[K][]A{}
	for _, item := range list {
		key := keyGetter(item)
		result[key] = append(result[key], item)
	}
	return result
}

func SortEachBucket[T comparable, A any](bucketedMap map[T][]A, f func(a A, b A) bool) map[T][]A {
	for k, v := range bucketedMap {
		sorted := SortBy(f, v)
		bucketedMap[k] = sorted
	}
	return bucketedMap
}

func GroupByE[T comparable, A any](list []A, f func(A) (T, error)) (map[T][]A, error) {
	result := map[T][]A{}
	for _, item := range list {
		key, err := f(item)
		if err != nil {
			return nil, errors.WrapAndTrace(err)
		}
		result[key] = append(result[key], item)
	}
	return result, nil
}

func Chunk[T any](list []T, chunkSize int) [][]T {
	var result [][]T
	for i := 0; i < len(list); i += chunkSize {
		end := i + chunkSize
		if end > len(list) {
			end = len(list)
		}
		result = append(result, list[i:end])
	}
	return result
}

func DecodeBase64OrValue(s string) (string, error) {
	if strings.HasPrefix(s, "base64:") {
		s = strings.TrimPrefix(s, "base64:")
		decoded, err := base64.StdEncoding.DecodeString(s)
		if err != nil {
			return "", errors.WrapAndTrace(err)
		}
		return string(decoded), nil
	}
	return s, nil
}

func LastXChars(s string, x int) string {
	if len(s) < x {
		return s
	}
	return s[len(s)-x:]
}

// creates a map of the keys in a and not in b
func MapDiff[K comparable, V1 any, V2 any](a map[K]V1, b map[K]V2) map[K]V1 {
	c := CloneMap(a)
	for k := range b {
		delete(c, k)
	}
	return c
}

func ListAnyDiff[T any, C comparable](a []T, b []T, toComp func(t T) C) []T {
	aM := ListToMap(a, toComp)
	bM := ListToMap(b, toComp)
	return GetMapValues(MapDiff(aM, bM))
}

func ListDiff[T comparable](a []T, b []T) []T {
	aCM := ListToCollisionMap(a)
	bCM := ListToCollisionMap(b)
	aNotB := MapDiff(aCM, bCM)
	return GetMapKeys(aNotB)
}

func CloneMap[T any, K comparable](m map[K]T) map[K]T {
	result := map[K]T{}
	for k, v := range m {
		result[k] = v
	}
	return result
}

func CloneList[T any](l []T) []T {
	result := []T{}
	for _, v := range l {
		result = append(result, v)
	}
	return result
}

func ReverseList[T any](list []T) {
	length := len(list)
	for i := 0; i < length/2; i++ {
		list[i], list[length-i-1] = list[length-i-1], list[i]
	}
}

func GetFirstKeyThatContainsNoCase[T any](m map[string]T, s string) string {
	s = strings.ToLower(s)
	for k := range m {
		if strings.Contains(strings.ToLower(k), s) {
			return k
		}
	}
	return ""
}

func Run[T any](l []T, f func(t T)) {
	for _, item := range l {
		f(item)
	}
}

func ParallelWorkerMapE[T, R any](items []T, mapper func(T) (R, error), maxWorkers int) ([]R, error) {
	var wg sync.WaitGroup
	var mu sync.Mutex

	length := len(items)
	results := make([]R, length)
	var allErr error

	// Create a buffered channel to act as a semaphore.
	semaphore := make(chan struct{}, maxWorkers)

	for idx, item := range items {
		// Acquire a token from the semaphore.
		semaphore <- struct{}{}

		wg.Add(1)
		go func(i int, itm T) {
			defer wg.Done()
			defer func() {
				if p := recover(); p != nil {
					// attach call stack to avoid missing in different goroutine
					mu.Lock()
					allErr = errors.Join(allErr, errors.Errorf("%+v\n\n%s", p, strings.TrimSpace(string(debug.Stack()))))
					mu.Unlock()
				}
			}()
			res, err := mapper(itm)
			mu.Lock()
			if err != nil {
				allErr = errors.Join(allErr, errors.Wrap(err, fmt.Sprint(i)))
			} else {
				results[i] = res
			}
			mu.Unlock()

			// Release a token back to the semaphore.
			<-semaphore
		}(idx, item)
	}

	wg.Wait()

	return results, allErr //nolint:wrapcheck // fine for internal
}

// Early returns if one error is found, will return partial work
func ParallelWorkerMapExitOnE[T, R any](ctx context.Context, items []T, mapper func(context.Context, T) (R, error), maxWorkers int) ([]R, error) {
	var wg sync.WaitGroup
	var mu sync.Mutex

	length := len(items)
	results := make([]R, length)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var firstErr error

	// Create a buffered channel to act as a semaphore.
	semaphore := make(chan struct{}, maxWorkers)

	for idx, item := range items {
		select {
		case <-ctx.Done():
		case semaphore <- struct{}{}:
			wg.Add(1)
			go func(i int, itm T) {
				defer wg.Done()
				defer func() { <-semaphore }()
				defer func() {
					if p := recover(); p != nil {
						err := errors.Errorf("%+v\n\n%s", p, strings.TrimSpace(string(debug.Stack())))
						mu.Lock()
						if firstErr == nil {
							firstErr = err
							cancel()
						}
						mu.Unlock()
					}
				}()

				if res, err := mapper(ctx, itm); err != nil {
					mu.Lock()
					if firstErr == nil {
						firstErr = err
						cancel()
					}
					mu.Unlock()
				} else {
					mu.Lock()
					results[i] = res
					mu.Unlock()
				}
			}(idx, item)
		}
	}

	wg.Wait()

	// Priority is given to firstErr if it's set
	if firstErr != nil {
		return results, errors.WrapAndTrace(firstErr)
	}

	// If firstErr is nil, then we check if the context was canceled
	return results, errors.WrapAndTrace(ctx.Err())
}

func Iterate[T any](hasNext func() bool, next func() (T, error), do func(a T) (bool, error)) error {
	for hasNext() {
		a, err := next()
		if err != nil {
			return errors.WrapAndTrace(err)
		}
		cont, err := do(a)
		if err != nil {
			return errors.WrapAndTrace(err)
		}
		if !cont {
			break
		}
	}
	return nil
}

// Assumes that if cont is false res is empty
func IterateToSlice[T any](f func() (bool, T, error)) ([]T, error) {
	allRes := []T{}
	for {
		cont, res, err := f()
		if err != nil {
			return nil, errors.WrapAndTrace(err)
		}
		if !cont {
			return allRes, nil
		}
		allRes = append(allRes, res)
	}
}

func RetryWithDataAndAttemptCount[T any](o backoff.OperationWithData[T], b backoff.BackOff) (T, error) {
	attemptCount := 0
	t, err := backoff.RetryWithData(func() (T, error) {
		attemptCount++
		return o()
	}, b)
	if err != nil {
		return t, errors.WrapAndTrace(errors.Errorf("attemptCount %d: %w", attemptCount, err))
	}
	return t, nil
}

func RetryWithAttemptCount(o backoff.Operation, b backoff.BackOff) error {
	attemptCount := 0
	err := backoff.Retry(func() error {
		attemptCount++
		return o()
	}, b)
	if err != nil {
		return errors.WrapAndTrace(errors.Errorf("attemptCount %d: %w", attemptCount, err))
	}
	return nil
}

type Runnable interface {
	Run(ctx context.Context) error
	Shutdown(ctx context.Context) error
}

// RunAllWithShutdown runs Runnabls in parallel and waits for shutdown signal (max n seconds)
// if one runner errors or panics
func RunAllWithShutdown(ctx context.Context, runners []Runnable, shutdownChan <-chan any) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var wgRun sync.WaitGroup
	errChan := make(chan error, len(runners))
	doneChan := make(chan struct{})
	for _, r := range runners {
		wgRun.Add(1)
		go func(runner Runnable) {
			defer wgRun.Done()
			defer func() {
				if p := recover(); p != nil {
					// attach call stack to avoid missing in different goroutine
					errChan <- errors.Errorf("%+v\n\n%s", p, strings.TrimSpace(string(debug.Stack())))
				}
			}()
			if err := runner.Run(ctx); err != nil {
				errChan <- err
			}
		}(r)
	}

	go func() {
		wgRun.Wait()
		close(doneChan)
	}()

	var runErr error
	select {
	case <-shutdownChan:
		// Received shutdown signal
	case err := <-errChan:
		// One of the Run methods returned an error
		runErr = err
	case <-doneChan:
		return nil
	}

	// Initiate shutdown of all runners
	var shutdownErrors error
	var mu sync.Mutex
	var wgShutdown sync.WaitGroup
	for _, r := range runners {
		wgShutdown.Add(1)
		go func(runner Runnable) {
			defer wgShutdown.Done()
			err := DoWithTimeout(func(ctx context.Context) error {
				err := runner.Shutdown(ctx)
				if err != nil {
					return errors.WrapAndTrace(err)
				}
				return nil
			}, 5*time.Second, TimeoutOptions{
				CatchPanic: true,
			})
			if err != nil {
				mu.Lock()
				shutdownErrors = errors.Join(shutdownErrors, err)
				mu.Unlock()
			}
		}(r)
	}

	wgShutdown.Wait()
	return errors.WrapAndTrace(errors.Join(runErr, shutdownErrors))
}

type ContextKey string

const IdempotencyKeyName ContextKey = "idempotencyKey"

func ContextWithIdempotencyKey(ctx context.Context, idempotencyKey string) context.Context {
	if idempotencyKey == "" {
		return ctx
	}
	return context.WithValue(ctx, IdempotencyKeyName, idempotencyKey)
}

func GetIdempotencyKeyFromContext(ctx context.Context) string {
	idempotencyKey, _ := ctx.Value(IdempotencyKeyName).(string) // do not check for error because if fail, then just leave as empty string
	return idempotencyKey
}

func MakePrefixIdempotencyKeyFromCtx(ctx context.Context, prefix string) string {
	key := GetIdempotencyKeyFromContext(ctx)
	if key == "" {
		return ""
	}
	return fmt.Sprintf("%s-%s", prefix, key)
}

// for testing, and printing to screen, ignores error
func ProtoToFormattedString(m proto.Message) string {
	res, _ := protojson.Marshal(m)
	var prettyJSON bytes.Buffer
	_ = json.Indent(&prettyJSON, res, "", "  ")
	return prettyJSON.String()
}

type SafeSlice[T any] struct {
	slice []T
	mu    sync.RWMutex
}

func NewSafeSlice[T any]() *SafeSlice[T] {
	return &SafeSlice[T]{
		slice: []T{},
		mu:    sync.RWMutex{},
	}
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
	for i, v := range s.slice {
		newSlice[i] = v
	}
	return newSlice
}

func SliceToSafeSet[T comparable](s []T) *SafeSet[T] {
	set := SafeSet[T]{}
	for _, item := range s {
		set.Add(item)
	}
	return &set
}

type SafeSet[K comparable] struct {
	m SafeMap[K, any]
}

func NewSafeSet[K comparable]() *SafeSet[K] {
	return &SafeSet[K]{
		m: *NewSafeMap[K, any](),
	}
}

func (s *SafeSet[K]) Add(key K) {
	if _, ok := s.m.Get(key); !ok {
		s.m.Set(key, nil)
	}
}

func (s *SafeSet[K]) Remove(key K) {
	s.m.Delete(key)
}

func (s *SafeSet[K]) Clear() {
	s.m.Clear()
}

func (s *SafeSet[K]) Contains(key K) bool {
	_, ok := s.m.Get(key)
	return ok
}

func (s *SafeSet[K]) Len() int {
	return s.m.Len()
}

func (s *SafeSet[K]) Values() []K {
	return s.m.Keys()
}

// SafeMap is a generic map guarded by a RW mutex.
type SafeMap[K comparable, V any] struct {
	mu    sync.RWMutex
	items map[K]V
}

// NewSafeMap creates a new SafeMap.
func NewSafeMap[K comparable, V any]() *SafeMap[K, V] {
	return &SafeMap[K, V]{
		items: map[K]V{},
	}
}

func (m *SafeMap[K, V]) Override(s map[K]V) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.items = s
}

// Set sets a value in the map.
func (m *SafeMap[K, V]) Set(key K, value V) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.items[key] = value
}

// Get retrieves a value from the map.
func (m *SafeMap[K, V]) Get(key K) (V, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	val, ok := m.items[key]
	return val, ok
}

// Delete removes a key from the map.
func (m *SafeMap[K, V]) Delete(key K) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.items, key)
}

// Clear removes all entries from the map.
func (m *SafeMap[K, V]) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.items = make(map[K]V)
}

func (m *SafeMap[K, V]) Values() []V {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return GetMapValues(m.items)
}

func (m *SafeMap[K, V]) Keys() []K {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return GetMapKeys(m.items)
}

func (m *SafeMap[K, V]) Len() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.items)
}

// Copy creates a new SafeMap with the same key-value pairs.
func (m *SafeMap[K, V]) Copy() *SafeMap[K, V] {
	m.mu.RLock()
	defer m.mu.RUnlock()

	newMap := NewSafeMap[K, V]()
	for key, value := range m.items {
		newMap.items[key] = value
	}
	return newMap
}

// Sets the pointer of the current map to a copy of the map passed in.
func (m *SafeMap[K, V]) CopyMap(c *SafeMap[K, V]) *SafeMap[K, V] {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.items = c.Copy().items
	return m
}

type SafeCounter struct {
	mu sync.Mutex
	c  int
}

func NewSafeCounter() *SafeCounter {
	return &SafeCounter{}
}

func (c *SafeCounter) Inc() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.c++
}

func (c *SafeCounter) Dec() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.c--
}

func (c *SafeCounter) Get() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.c
}

// SafeValue holds an arbitrary value with read and write protection.
// T is the type of the value.
type SafeValue[T any] struct {
	value T
	mutex sync.RWMutex
}

// NewSafeValue creates a new SafeValue.
func NewSafeValue[T any](initialValue T) *SafeValue[T] {
	return &SafeValue[T]{
		value: initialValue,
	}
}

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

func RoundToNearestBase(num float64, base float64) float64 {
	return math.Round(num/base) * base
}

func RoundToNearestDecimal(num float64, decimalPlaces int) float64 {
	shift := math.Pow(10, float64(decimalPlaces))
	return math.Round(num*shift) / shift
}

func ListToChannel[T any](l []T) chan T {
	c := make(chan T, len(l))
	for _, item := range l {
		c <- item
	}
	close(c)
	return c
}

func DistinctChan[T any, K comparable](
	keyGetter func(T any) K, bufSize int,
) (input chan T, output chan T) {
	input = make(chan T, bufSize)
	output = make(chan T, bufSize)

	go func() {
		set := make(map[K]T)
		for i := range input {
			k := keyGetter(i)
			if _, ok := set[k]; !ok {
				set[k] = i
				output <- i
				delete(set, k)
			}
		}
		close(output)
	}()
	return
}

func GetFunctionName(i any) string {
	nameStr, ok := i.(string)
	if ok {
		return nameStr
	}
	return runtime.FuncForPC(reflect.ValueOf(i).Pointer()).Name()
}

func GetFieldName(structPtr any, fieldPtr any) string {
	res := findStructField(reflect.ValueOf(structPtr).Elem(), reflect.ValueOf(fieldPtr))
	return res.Name
}

// findStructField looks for a field in the given struct.
// The field being looked for should be a pointer to the actual struct field.
// If found, the field info will be returned. Otherwise, nil will be returned.
func findStructField(structValue reflect.Value, fieldValue reflect.Value) *reflect.StructField {
	ptr := fieldValue.Pointer()
	for i := structValue.NumField() - 1; i >= 0; i-- {
		sf := structValue.Type().Field(i)
		if ptr == structValue.Field(i).UnsafeAddr() {
			// do additional type comparison because it's possible that the address of
			// an embedded struct is the same as the first field of the embedded struct
			if sf.Type == fieldValue.Elem().Type() {
				return &sf
			}
		}
		if sf.Anonymous {
			// delve into anonymous struct to look for the field
			fi := structValue.Field(i)
			if sf.Type.Kind() == reflect.Ptr {
				fi = fi.Elem()
			}
			if fi.Kind() == reflect.Struct {
				if f := findStructField(fi, fieldValue); f != nil {
					return f
				}
			}
		}
	}
	return nil
}

// returns json string or empty if fails
func ToJSONString(i any) string {
	b, _ := json.Marshal(i)
	return string(b)
}

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

func NewRollback(undo func() error) *Rollback {
	return &Rollback{undos: []func() error{undo}}
}

func (r *Rollback) Add(undo func() error) {
	r.undos = append(r.undos, undo)
}

func (r Rollback) Rollback() error {
	var err error
	for i := len(r.undos) - 1; i >= 0; i-- {
		if e := r.undos[i](); e != nil {
			err = errors.Join(err, e)
		}
	}
	return err //nolint:wrapcheck // fine for internal
}

// SleepWithHealthCheck sleeps for the specified duration `d` and periodically calls `heartbeatFn`
// at every `tickRate` until `d` has elapsed.
func SleepWithHealthCheck(d time.Duration, tickRate time.Duration, heartbeatFn func()) {
	heartbeatFn() // Call the heartbeat function immediately
	// Timer to manage the total sleep duration
	sleepTimer := time.NewTimer(d)
	// Ticker to manage the heartbeat function calls
	tickTicker := time.NewTicker(tickRate)
	defer tickTicker.Stop() // Ensures the ticker is stopped to free resources

	go func() {
		for {
			select {
			case <-tickTicker.C: // On every tick, call the heartbeat function
				heartbeatFn()
			case <-sleepTimer.C: // Once the total duration has passed, return
				heartbeatFn()
				return
			}
		}
	}()

	<-sleepTimer.C // Wait for the sleep duration to pass before returning
}

func OnTick(ctx context.Context, d time.Duration, f func()) *time.Ticker {
	ticker := time.NewTicker(d)
	go func() {
		for {
			select {
			case <-ctx.Done():
				ticker.Stop()
				return
			case <-ticker.C:
				f()
			}
		}
	}()
	return ticker
}

// cancel context to end
func DoForever(ctx context.Context, f func()) {
	_ = DoForeverE(ctx,
		func() error {
			f()
			return nil
		}, func(_ context.Context) error {
			return nil
		})
}

func DoForeverE(ctx context.Context,
	f func() error,
	done func(context.Context) error,
) error {
	for {
		select {
		case <-ctx.Done():
			return done(ctx)
		default:
			return f()
		}
	}
}

func DoAfterE(ctx context.Context, d time.Duration,
	f func() error,
	done func(context.Context) error,
) error {
	for {
		select {
		case <-ctx.Done():
			return done(ctx)
		case <-time.After(d):
			return f()
		}
	}
}

// cancel context to end
func DoOnDuration(ctx context.Context, d time.Duration, f func()) {
	ticker := time.NewTicker(d)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			f()
		}
	}
}

// end early if err
func DoOnDurationE(ctx context.Context, d time.Duration,
	f func() error,
	done func(context.Context) error,
) error {
	ticker := time.NewTicker(d)
	for {
		select {
		case <-ctx.Done():
			return done(ctx)
		case <-ticker.C:
			err := f()
			if err != nil {
				return err
			}
		}
	}
}

type UniqueBufferedObjects[T any] struct {
	bufferedObjects *BufferedObjects[T]
	getID           func(T) string
	objectsInBuffer *SafeSet[string]
}

func NewUniqueBufferedObjects[T any](
	flushSize int,
	flushInterval time.Duration,
	getID func(T) string,
	flushHandler func([]T),
) *UniqueBufferedObjects[T] {
	ubo := &UniqueBufferedObjects[T]{
		getID:           getID,
		objectsInBuffer: NewSafeSet[string](),
	}
	bo := NewBufferedObjects(flushSize, flushInterval, func(t []T) {
		flushHandler(t)
		for _, item := range t {
			ubo.objectsInBuffer.Remove(getID(item))
		}
	})
	ubo.bufferedObjects = bo
	return ubo
}

func (bi *UniqueBufferedObjects[T]) Add(object T) {
	objID := bi.getID(object)
	if bi.objectsInBuffer.Contains(objID) {
		return
	}
	bi.objectsInBuffer.Add(objID)
	bi.bufferedObjects.Add(object)
}

func (bi *UniqueBufferedObjects[T]) Flush() {
	bi.bufferedObjects.Flush()
}

func (bi *UniqueBufferedObjects[T]) Stop() {
	bi.bufferedObjects.Stop()
}

func (bi *UniqueBufferedObjects[T]) WaitTillEmpty() {
	bi.bufferedObjects.WaitTillEmpty()
}

type BufferedObjects[T any] struct {
	objects       []T
	lock          sync.Mutex
	flushSize     int
	flushInterval time.Duration
	flushChan     chan []T
	stopChan      chan struct{}
	handlingFlush SafeValue[bool]
}

func NewBufferedObjects[T any](flushSize int, flushInterval time.Duration, flushHandler func([]T)) *BufferedObjects[T] {
	bi := &BufferedObjects[T]{
		flushSize:     flushSize,
		flushInterval: flushInterval,
		flushChan:     make(chan []T, 100), //  [][]T buffer
		stopChan:      make(chan struct{}),
	}
	go bi.run(flushHandler)
	return bi
}

func (bi *BufferedObjects[T]) SetBufferSize(flushSize int) {
	bi.flushChan = make(chan []T, flushSize)
}

func (bi *BufferedObjects[T]) Add(object T) {
	bi.lock.Lock()
	defer bi.lock.Unlock()

	bi.objects = append(bi.objects, object)
	if len(bi.objects) >= bi.flushSize {
		bi.Flush()
	}
}

func (bi *BufferedObjects[T]) Flush() {
	// Copy and reset buffer under lock to minimize lock time
	toFlush := make([]T, len(bi.objects))
	copy(toFlush, bi.objects)
	bi.objects = nil

	// Send to flush channel
	bi.flushChan <- toFlush
}

func (bi *BufferedObjects[T]) WaitTillEmpty() {
	for {
		bi.lock.Lock()
		if len(bi.objects) == 0 && len(bi.flushChan) == 0 && !bi.handlingFlush.Get() {
			bi.lock.Unlock()
			return
		}
		bi.lock.Unlock()
		time.Sleep(50 * time.Millisecond)
	}
}

func (bi *BufferedObjects[T]) run(flushHandler func([]T)) {
	ticker := time.NewTicker(bi.flushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			bi.lock.Lock()
			if len(bi.objects) > 0 {
				bi.Flush()
			}
			bi.lock.Unlock()
		case objects := <-bi.flushChan:
			bi.handlingFlush.Set(true)
			flushHandler(objects)
			bi.handlingFlush.Set(false)
		case <-bi.stopChan:
			return
		}
	}
}

func (bi *BufferedObjects[T]) Stop() {
	close(bi.stopChan)
}

func ContainsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

func RetryTest(t *testing.T, testFunc func(t *testing.T), numRetries int) {
	t.Helper() // Mark this function as a helper
	for i := 0; i < numRetries; i++ {
		tt := &testing.T{}
		testFunc(tt)
		if !tt.Failed() {
			return
		}
	}
	t.Fail() // If we reach here, all retries failed
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

// if you loop forever, make sure you have a way to break the loop
// see Test_DoWithTimeoutTimeoutLoop
func DoWithTimeout(fn func(ctx context.Context) error, timeout time.Duration, opts ...TimeoutOptions) error {
	_, err := DoWithTimeoutData(func(ctx context.Context) (interface{}, error) {
		return nil, fn(ctx)
	}, timeout, opts...)
	return err
}

// if you loop forever, make sure you have a way to break the loop
// see Test_DoWithTimeoutTimeoutLoop
func DoWithTimeoutData[T any](fn func(ctx context.Context) (T, error), timeout time.Duration, opts ...TimeoutOptions) (T, error) {
	type result struct {
		res T
		err error
	}
	options := TimeoutOptions{
		ParentContext: context.Background(),
	}
	for _, opt := range opts {
		options = opt
	}
	if options.ParentContext == nil {
		options.ParentContext = context.Background()
	}
	ctx, cancel := context.WithTimeout(options.ParentContext, timeout)
	defer cancel()

	// create channel with buffer size 1 to avoid goroutine leak
	resChan := make(chan result, 1)
	panicChan := make(chan interface{}, 1)
	go func() {
		defer func() {
			if p := recover(); p != nil {
				// attach call stack to avoid missing in different goroutine
				panicChan <- fmt.Sprintf("%+v\n\n%s", p, strings.TrimSpace(string(debug.Stack())))
			}
		}()
		res, err := fn(ctx)
		resChan <- result{res, err}
	}()

	var emptyT T

	select {
	case p := <-panicChan:
		if options.CatchPanic {
			return emptyT, fmt.Errorf("panic: %v", p)
		} else {
			panic(p)
		}
	case result := <-resChan:
		return result.res, result.err
	case <-ctx.Done():
		return emptyT, ctx.Err() //nolint:wrapcheck // no need to wrap
	}
}

// WithContext customizes a DoWithTimeout call with given ctx.
func WithContext(ctx context.Context) DoOption {
	return func() context.Context {
		return ctx
	}
}
