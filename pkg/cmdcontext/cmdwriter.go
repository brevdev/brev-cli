package cmdcontext

// NoopWriter is an implementation of the standard Writer which takes no action
// upon being asked to write.
type NoopWriter struct{}

func (w NoopWriter) Write(_ []byte) (n int, err error) {
	return 0, nil
}
