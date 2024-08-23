package prettyconsole

import (
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"

	"github.com/pkg/errors"
)

var reErrorJoins = regexp.MustCompile(`[\s,:;\\n]+$`)

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
		_, _ = e.buf.Write(enc.buf.Bytes()) // Explicitly ignore errors
		putPrettyConsoleEncoder(enc)

		e.inList = true
		e.listSep = e.cfg.LineEnding + strings.Repeat(" ", e.namespaceIndent)
	}()

	var causes []error
	switch et := err.(type) {
	case interface{ Errors() []error }:
		causes = et.Errors()
	case interface{ Unwrap() []error }:
		causes = et.Unwrap()
	case interface{ Cause() error }:
		causes = []error{et.Cause()}
	case interface{ Unwrap() error }:
		causes = []error{et.Unwrap()}
	}

	basic := err.Error()
	for _, cause := range causes {
		if cause != nil {
			cbasic := cause.Error()
			basic, _, _ = strings.Cut(basic, cbasic)
			// TrimSuffix with seperator characters like : or , surrounded by
			// any number of spaces
			basic = reErrorJoins.ReplaceAllString(strings.TrimSpace(basic), "")
		}
	}
	if basic != "" {
		enc.namespaceIndent += 1
		enc.colorizeAtLevel("=")
		enc.inList = true
		enc.addSafeString(basic)
	}

	// Write causes recursively
	skipDetail := false
	for i, ei := range causes {
		if ei == nil {
			continue
		}
		if len(causes) > 1 {
			key = "cause." + strconv.Itoa(i)
		} else {
			key = "cause"
		}
		if err := enc.encodeError(key, ei); err != nil {
			return err
		}
		skipDetail = true
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
