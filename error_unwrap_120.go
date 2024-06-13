//go:build go1.20

package prettyconsole

import (
	"errors"
	"reflect"
	"strconv"
)

func writeError(enc *prettyConsoleEncoder, err error) (bool, error) {
	// Write causes recursively
	skipDetail := false
	switch et := err.(type) {
	case interface{ Cause() error }:
		enc.addSafeString(err.Error())
		if err := enc.encodeError("cause", et.Cause()); err != nil {
			return skipDetail, err
		}
		skipDetail = true

	case interface{ Errors() []error }:
		enc.addSafeString(err.Error())
		for i, ei := range et.Errors() {
			if err := enc.encodeError("cause."+strconv.Itoa(i), ei); err != nil {
				return skipDetail, err
			}
			skipDetail = true
		}

	case interface{ Unwrap() []error }:
		if reflect.TypeOf(err) != joinErrorType {
			// Special case the joinError type from the errors package, since its message is just the
			// concatenation of its children separated by newlines (not something nicer like ':').
			// Other notable types that implement this include fmt.Errorf with >1 %w verbs; however,
			// because the message could contain other pertinent information, we don't want to skip.
			enc.addSafeString(err.Error())
		}
		for i, ei := range et.Unwrap() {
			if err := enc.encodeError(strconv.Itoa(i), ei); err != nil {
				return skipDetail, err
			}
			skipDetail = true
		}

	default:
		enc.addSafeString(err.Error())
	}
	return skipDetail, nil
}

// joinErrorType is the reflect.Type of the joinError type from the errors package. Because it's a private struct,
// we have to use reflect on the result of the function to check for it. `Join` on 0-1 non-nil errors won't give this struct,
// so use 2 arbitrary non-nil errors to get it.
var joinErrorType = reflect.TypeOf(errors.Join(errors.ErrUnsupported, errors.ErrUnsupported))
