package errors

import (
	"fmt"
	"testing"

	pkgerrors "github.com/pkg/errors"

	"github.com/stretchr/testify/assert"
)

// Learn about how errors work in Go
// Errors are surprisingly complex in Go due to
// different implementations at different times
// TLDR; use Cause and Unwrap with CAUTION
// Cause seems deprecated. Unwrap only unwraps once which may be unexpected.
// Always use Is and As
// %+v for verbose error printing (stacktrace etc)

func Test_errorNew(t *testing.T) {
	err := New("my error")
	assert.Equal(t, "my error", err.Error())

	// simple error assertion
	assert.True(t, Is(err, err))
	otherErr := New("other error")
	assert.False(t, Is(err, otherErr))

	// error assertion with wrap (checks through error chain)
	wrappedErr := Wrap(err, "wrap message")
	assert.True(t, Is(wrappedErr, err))
}

type MyError struct {
	SomeValue int
}

func (e MyError) Error() string {
	return fmt.Sprintf("my error: %d", e.SomeValue)
}

func Test_customError(t *testing.T) {
	errVal1 := MyError{SomeValue: 1}
	errVal2 := MyError{SomeValue: 2}

	assert.False(t, Is(errVal1, errVal2))
	assert.False(t, Is(errVal1, &MyError{}))
	assert.False(t, Is(errVal1, MyError{}))

	assert.True(t, As(errVal1, &MyError{}))
	assert.True(t, As(errVal2, &MyError{}))
	// need to pass in ptr
	assert.Panics(t, func() {
		assert.False(t, As(errVal1, MyError{}))
	})

	// error assertion with wrap (checks through error chain)
	wrappedErr := Wrap(errVal1, "wrap message")
	assert.True(t, Is(wrappedErr, errVal1))
	assert.True(t, As(wrappedErr, &MyError{}))
}

func Test_StdWrapError(t *testing.T) {
	err := New("my error")
	assert.IsType(t, err, Errorf("my error"))
	assert.NotSame(t, err, pkgerrors.Errorf("my error")) // NotType does not exist

	wrap1 := Wrap(err, "wrap 1")
	wrap2 := Wrap(wrap1, "wrap 2")
	assert.True(t, Is(wrap2, err))
	assert.Equal(t, "wrap 2: wrap 1: my error", wrap2.Error())
	fmt.Println("verbose err:")
	fmt.Printf("%+v\n", wrap2) // print stacktrace

	assert.NotEqual(t, err, pkgerrors.Cause(wrap2)) // cause NOT work
	assert.Equal(t, wrap1, Unwrap(wrap2))           // "should" get 1 level unwrap
	assert.NotEqual(t, pkgerrors.Cause(wrap2), Unwrap(wrap2))
}

func Test_Errorf(t *testing.T) {
	err := Errorf("my error")
	assert.NotSame(t, err, Errorf("my error"))
	assert.NotSame(t, err, New("my error"))

	// using different string format "|" to demonstrate flexibility fo Errorf()
	wrap1 := Errorf("wrap 1| %w", err)
	wrap2 := Errorf("wrap 2| %w", wrap1)
	assert.True(t, Is(wrap2, err))
	assert.Equal(t, "wrap 2| wrap 1| my error", wrap2.Error())
	fmt.Println("verbose err:")
	fmt.Printf("%+v\n", wrap2) // does not print stacktrace

	assert.NotEqual(t, err, pkgerrors.Cause(wrap2)) // cause does not work
	assert.Equal(t, wrap2, pkgerrors.Cause(wrap2))  // returns original error
	assert.Equal(t, wrap1, Unwrap(wrap2))           // unwrap works
}

// func Test_ReturnWrapError(t *testing.T) {
// 	err := New("my error")

// 	wrap1 := errtrace.Wrap(err) // Notice, no message
// 	wrap2 := errtrace.Wrap(wrap1)

// 	assert.True(t, Is(wrap2, err))
// 	assert.Equal(t, "my error", wrap2.Error())
// 	fmt.Println("verbose err:")
// 	fmt.Printf("%+v\n", wrap2) // print return trace

// 	assert.NotEqual(t, err, pkgerrors.Cause(wrap2)) // cause does not work
// 	assert.Equal(t, wrap2, pkgerrors.Cause(wrap2))  // returns original error
// 	assert.Equal(t, wrap1, Unwrap(wrap2))           // unwrap works
// }

func Test_WrapAndTraceError(t *testing.T) {
	err := New("my error")

	wrap1 := WrapAndTrace(Errorf("wrap 1: %w", err))
	wrap2 := WrapAndTrace(Errorf("wrap 2: %w", wrap1))

	assert.True(t, Is(wrap2, err))
	// assert.Equal(t, "wrap 2: wrap 1: my error", wrap2.Error()) // includes line nums
	fmt.Println("verbose err:")
	fmt.Printf("%+v\n", wrap2) // print stacktrace

	assert.NotEqual(t, err, pkgerrors.Cause(wrap2)) // cause does NOT work
	assert.Equal(t, err, Root(wrap2))               // root works
}

// append we use
func Test_JoinError(t *testing.T) {
	err1 := New("my error 1")
	err2 := New("my error 2")
	err3 := New("my error 3")

	joinedErr := Join(err1, err2, err3)
	assert.Equal(t, "my error 1\nmy error 2\nmy error 3", joinedErr.Error())
	fmt.Println("verbose err:")
	fmt.Printf("%+v\n", joinedErr)

	assert.True(t, Is(joinedErr, err1))
	assert.True(t, Is(joinedErr, err2))
	assert.True(t, Is(joinedErr, err3))
	assert.Equal(t, joinedErr, pkgerrors.Cause(joinedErr)) // cause does not work
	assert.Nil(t, Unwrap(joinedErr))                       // unwrap does not work
	assert.Len(t, Unwraps(joinedErr), 3)
	assert.Equal(t, joinedErr, Root(joinedErr))

	wrappedErr3 := Wrap(err3, "wrap 3")
	joinedWithWrapErr := Join(err1, err2, wrappedErr3)
	assert.Equal(t, joinedErr.Error(), Root(joinedWithWrapErr).Error())

	// errorJoinedNil := Join(nil, err1)
	// errs := Unwraps(errorJoinedNil)
	// assert.Len(t, errs, 1)

	// errorJoinedNil = Join(err1, nil)
	// errs = Unwraps(errorJoinedNil)
	// assert.Len(t, errs, 1)

	errorJoinedNil := Join(nil, nil)
	assert.Nil(t, errorJoinedNil)
}

// what happens when we combine them
func Test_ErrorfWithWrap(t *testing.T) {
	// errorf unwrap works cause doesn't
	// wrap unwrap requires 2 unwraps and cause works

	err := New("my error")

	wrap1 := Wrap(err, "wrap 1")
	wrap2 := Errorf("wrap 2| %w", wrap1)

	assert.True(t, Is(wrap2, err))
	assert.Equal(t, "wrap 2| wrap 1: my error", wrap2.Error())
	fmt.Println("verbose err:")
	fmt.Printf("%+v\n", wrap2) // print stacktrace

	assert.NotEqual(t, err, pkgerrors.Cause(wrap2)) // cause does NOT work
	assert.Equal(t, wrap1, Unwrap(wrap2))           // unwrap works

	// change order
	wrap1 = Errorf("wrap 1| %w", err)
	wrap2 = Wrap(wrap1, "wrap 2")

	assert.True(t, Is(wrap2, err))
	assert.Equal(t, "wrap 2: wrap 1| my error", wrap2.Error())

	assert.NotEqual(t, err, pkgerrors.Cause(wrap2))   // cause does NOT work
	assert.NotEqual(t, wrap1, pkgerrors.Cause(wrap2)) // cause does NOT work
	assert.Equal(t, wrap1, Unwrap(wrap2))             // unwrap does NOT work
}

// what happens when we use the errors package with our errors
func Test_AppendWithWrap(t *testing.T) {
	err := New("my error")

	wrap1 := Wrap(err, "wrap 1")

	err2 := New("my error 2")

	joinedErr := Join(wrap1, err2)

	wrappedAppendErr := Wrap(joinedErr, "wrap 2")

	assert.True(t, Is(wrappedAppendErr, err))
	assert.Equal(t, "wrap 2: wrap 1: my error\nmy error 2", wrappedAppendErr.Error())
	fmt.Println("verbose err:")
	fmt.Printf("%+v\n", wrappedAppendErr) // print stacktrace

	assert.NotEqual(t, err, pkgerrors.Cause(wrappedAppendErr)) // cause does NOT works
	assert.NotEqual(t, joinedErr, pkgerrors.Cause(wrappedAppendErr))

	assert.Equal(t, joinedErr, Unwrap(wrappedAppendErr)) // unwrap does NOT work
}

func Test_Root(t *testing.T) {
	err := New("my error")

	wrap1 := Wrap(err, "wrap 1")
	wrap2 := Wrap(wrap1, "wrap 2")

	assert.Equal(t, err, Root(wrap2))
	assert.Equal(t, err, Root(wrap1))
	assert.Equal(t, err, Root(err))
}

func Test_combine(t *testing.T) {
	err1 := New("my error 1")

	errs := Join(err1, err1)
	errs = Join(errs, err1)
	cerr := CombineByString(errs)

	assert.Equal(t, "my error 1", cerr.Error())
}
