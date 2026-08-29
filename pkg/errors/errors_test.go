package errors

import (
	stderrors "errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
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

func Test_IsNetworkError(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"plain error", New("oops"), false},
		{"dns error", &net.DNSError{Name: "example.invalid", Err: "no such host"}, true},
		{"op error", &net.OpError{Op: "dial", Net: "tcp", Err: stderrors.New("connection refused")}, true},
		{"url error wrapping op error", &url.Error{
			Op:  "Get",
			URL: "https://api.ngc.nvidia.com/token",
			Err: &net.OpError{Op: "dial", Net: "tcp", Err: stderrors.New("connection refused")},
		}, true},
		{"wrapped dns error", fmt.Errorf("outer: %w", &net.DNSError{Name: "example.invalid", Err: "no such host"}), true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, IsNetworkError(tc.err))
		})
	}
}

func Test_HostFromURLError(t *testing.T) {
	urlErr := &url.Error{Op: "Get", URL: "https://api.ngc.nvidia.com/token", Err: stderrors.New("boom")}
	assert.Equal(t, "api.ngc.nvidia.com", HostFromURLError(urlErr))

	// Wrapped url.Error is also handled.
	wrapped := fmt.Errorf("outer: %w", urlErr)
	assert.Equal(t, "api.ngc.nvidia.com", HostFromURLError(wrapped))

	// Non-url.Error returns empty.
	assert.Equal(t, "", HostFromURLError(stderrors.New("plain")))
	assert.Equal(t, "", HostFromURLError(nil))
}

func Test_WrapNetworkError(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		assert.Nil(t, WrapNetworkError(nil, "host"))
	})

	t.Run("non-network error passes through", func(t *testing.T) {
		orig := New("not a network error")
		result := WrapNetworkError(orig, "host")
		assert.Equal(t, orig, result)
		var netErr *NetworkError
		assert.False(t, stderrors.As(result, &netErr), "non-network error should not be wrapped")
	})

	t.Run("network error gets wrapped with host from url.Error", func(t *testing.T) {
		inner := &url.Error{
			Op:  "Get",
			URL: "https://api.ngc.nvidia.com/token",
			Err: &net.OpError{Op: "dial", Net: "tcp", Err: stderrors.New("connection refused")},
		}
		wrapped := WrapNetworkError(inner, "fallback.example.com")

		var netErr *NetworkError
		if assert.True(t, stderrors.As(wrapped, &netErr)) {
			assert.Equal(t, "api.ngc.nvidia.com", netErr.Host)
			assert.Same(t, inner, netErr.Cause)
		}
	})

	t.Run("network error uses fallback host when no url.Error in chain", func(t *testing.T) {
		inner := &net.OpError{Op: "dial", Net: "tcp", Err: stderrors.New("connection refused")}
		wrapped := WrapNetworkError(inner, "fallback.example.com")

		var netErr *NetworkError
		if assert.True(t, stderrors.As(wrapped, &netErr)) {
			assert.Equal(t, "fallback.example.com", netErr.Host)
		}
	})
}

func Test_NetworkError_Messages(t *testing.T) {
	withHost := &NetworkError{Host: "api.ngc.nvidia.com"}
	assert.Contains(t, withHost.Error(), "api.ngc.nvidia.com")
	assert.Contains(t, withHost.Error(), "internet connection")
	assert.Contains(t, withHost.Directive(), "api.ngc.nvidia.com")

	withoutHost := &NetworkError{}
	assert.Contains(t, withoutHost.Error(), "internet connection")
	assert.NotEmpty(t, withoutHost.Directive())
}

func Test_NetworkError_UnwrapToCause(t *testing.T) {
	cause := stderrors.New("boom")
	netErr := &NetworkError{Host: "h", Cause: cause}
	assert.Same(t, cause, netErr.Unwrap())
	assert.True(t, stderrors.Is(netErr, cause))
}

// Verifies the integration path the CLI exercises: a real http.Client.Do
// against a closed listener returns an error that IsNetworkError detects
// and WrapNetworkError converts into a *NetworkError with the right host.
func Test_WrapNetworkError_RealHTTPClient(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	server.Close()

	resp, err := http.Get(server.URL) //nolint:noctx,bodyclose // test
	if resp != nil {
		_ = resp.Body.Close()
	}
	if !assert.Error(t, err) {
		return
	}
	assert.True(t, IsNetworkError(err), "expected closed-server error to be classified as network error: %v", err)

	wrapped := WrapNetworkError(err, "")
	var netErr *NetworkError
	if assert.True(t, stderrors.As(wrapped, &netErr)) {
		// Host should be derived from the url.Error embedded in err.
		expectedHost, _ := url.Parse(server.URL)
		assert.Equal(t, expectedHost.Host, netErr.Host)
	}
}

// pkgerrors.Cause must walk through WrapAndTrace layers and stop at the
// NetworkError so the cmd-level error renderer can detect it.
func Test_NetworkError_SurvivesWrapAndTrace(t *testing.T) {
	netErr := &NetworkError{Host: "api.ngc.nvidia.com", Cause: stderrors.New("boom")}
	wrapped := WrapAndTrace(WrapAndTrace(netErr))
	cause := pkgerrors.Cause(wrapped)
	assert.IsType(t, &NetworkError{}, cause)
}
