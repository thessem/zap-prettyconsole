//go:build !go1.20

package prettyconsole

import "strconv"

func writeError(enc *prettyConsoleEncoder, err error) (bool, error) {
	basic := err.Error()
	enc.addSafeString(basic)

	skipDetail := false
	switch et := err.(type) {
	case interface{ Cause() error }:
		if err := enc.encodeError("cause", et.Cause()); err != nil {
			return skipDetail, err
		}
		skipDetail = true

	case interface{ Errors() []error }:
		for i, ei := range et.Errors() {
			if err := enc.encodeError("cause."+strconv.Itoa(i), ei); err != nil {
				return skipDetail, err
			}
			skipDetail = true
		}
	}
	return skipDetail, nil
}
