package prettyconsole

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/pkg/errors"
)

// Encodes the given error into fields of an object. A field with the given
// name is added for the error message.
//
// If the error implements fmt.Formatter, a field with the name ${key}Verbose
// is also added with the full verbose error message.
//
// Finally, if the error implements errorGroup (from go.uber.org/multierr) or
// causer (from github.com/pkg/errors), a ${key}Causes field is added with an
// array of objects containing the errors this error was comprised of.
//
//	{
//	  "error": err.Error(),
//	  "errorVerbose": fmt.Sprintf("%+v", err),
//	  "errorCauses": [
//	    ...
//	  ],
//	}
func (e *prettyConsoleEncoder) encodeError(key string, err error) (retErr error) {
	enc := e.clone()
	enc.OpenNamespace(key)

	enc.colorizeAtLevel("=")
	enc.namespaceIndent += 1
	enc.inList = true

	// Try to capture panics (from nil references or otherwise) when calling
	// the Error() method
	defer func() {
		if rerr := recover(); rerr != nil {
			// If it's a nil pointer, just say "<nil>". The likeliest causes are a
			// error that fails to guard against nil or a nil pointer for a
			// value receiver, and in either case, "<nil>" is a nice result.
			if v := reflect.ValueOf(err); v.Kind() != reflect.Ptr || v.IsNil() {
				retErr = fmt.Errorf("PANIC=%v", rerr)
				putPrettyConsoleEncoder(enc)
				return
			}
			enc.addSafeString("<nil>")
		}
		e.buf.Write(enc.buf.Bytes())
		putPrettyConsoleEncoder(enc)

		e.inList = true
		e.listSep = "\n" + strings.Repeat(" ", e.namespaceIndent)
	}()

	basic := err.Error()
	enc.addSafeString(basic)

	// Write causes recursively
	skipDetail := false
	switch et := err.(type) {
	case interface{ Cause() error }:
		if err := enc.encodeError("cause", et.Cause()); err != nil {
			return err
		}
		skipDetail = true
	case interface{ Errors() []error }:
		for i, ei := range et.Errors() {
			if err := enc.encodeError("cause."+strconv.Itoa(i), ei); err != nil {
				return err
			}
			skipDetail = true
		}
	}

	// If there's a stacktrace, print it. If this error is a formatter, we'll
	// print the detail unless we extracted sub-errors above (as we're probably
	// just reprinting information we already extracted).
	if st, ok := err.(interface{ StackTrace() errors.StackTrace }); ok {
		enc.OpenNamespace("")
		enc.namespaceIndent += len("stacktrace=")
		enc.addIndentedString("stacktrace", strings.TrimPrefix(fmt.Sprintf("%+v", st.StackTrace()), "\n"))
	} else if ef, ok := err.(fmt.Formatter); ok && !skipDetail {
		enc.OpenNamespace("")
		enc.namespaceIndent += len("detail=")
		enc.addIndentedString("detail", strings.TrimPrefix(fmt.Sprintf("%+v", ef), "\n"))
	}

	// Normal clean up is actually in the defer

	return nil
}
