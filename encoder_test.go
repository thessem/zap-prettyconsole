package prettyconsole

import (
	"bytes"
	"errors"
	"fmt"
	"log"
	"regexp"
	"sort"
	"testing"
	"time"

	pkgerrors "github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"go.uber.org/multierr"
	"go.uber.org/zap"
	"go.uber.org/zap/buffer"
	"go.uber.org/zap/zapcore"
)

func TestEncodeEntry(t *testing.T) {
	// Remove stacktrace line-numbers from this test file. Remember to manually
	// test with -trimpath
	rPath := regexp.MustCompile(`(github|\/|testing|runtime)[\w\.\\\/\-]*:\d+`)

	tests := []struct {
		desc     string
		expected string
		ent      zapcore.Entry
		fields   []zapcore.Field
	}{
		{
			desc: "Minimal",
			// 4:33PM INF >
			expected: "\x1b[90m4:33PM\x1b[0m\x1b[32m \x1b[0m\x1b[32mINF\x1b[0m\x1b[32m \x1b[0m\x1b[1m\x1b[32m>\x1b[0m\x1b[0m\n",
			ent: zapcore.Entry{
				Level: zap.InfoLevel,
				Time:  time.Date(2018, 6, 19, 16, 33, 42, 99, time.UTC),
			},
			fields: []zapcore.Field{},
		},
		{
			desc: "Basic",
			// 4:33PM INF TestLogger ../<some_file>:<line_number> > log\nmessage complex=-8+12i duration=3h0m0s float=-30000000000000 int=0 string=test_\n_value time=2022-06-19T16:33:42Z
			//   ↳ strings=[\u001b1, 2\t]
			expected: "\x1b[90m4:33PM\x1b[0m\x1b[32m \x1b[0m\x1b[32mINF\x1b[0m\x1b[32m \x1b[0m\x1b[1mTestLogger\x1b[0m\x1b[32m \x1b[0m\x1b[1m\x1b[32m>\x1b[0m\x1b[0m\x1b[32m \x1b[0mlog\x1b[32m\\n\x1b[0mmessage\x1b[32m \x1b[0m\x1b[32mcomplex=\x1b[0m-8+12i\x1b[32m \x1b[0m\x1b[32mduration=\x1b[0m3h0m0s\x1b[32m \x1b[0m\x1b[32mfloat=\x1b[0m-30000000000000\x1b[32m \x1b[0m\x1b[32mint=\x1b[0m0\x1b[32m \x1b[0m\x1b[32mstring=\x1b[0mtest_\x1b[32m\\n\x1b[0m_value\x1b[32m \x1b[0m\x1b[32mtime=\x1b[0m2022-06-19T16:33:42Z\n\x1b[32m  ↳ strings\x1b[0m\x1b[32m=[\x1b[0m\x1b[32m\\u00\x1b[0m\x1b[32m1\x1b[0m\x1b[32mb\x1b[0m1\x1b[32m, \x1b[0m2\x1b[32m\\t\x1b[0m\x1b[32m]\x1b[0m\n",
			ent: zapcore.Entry{
				Level:      zap.InfoLevel,
				Time:       time.Date(2018, 6, 19, 16, 33, 42, 99, time.UTC),
				LoggerName: "TestLogger",
				Message:    "log\nmessage",
				Caller:     zapcore.NewEntryCaller(100, "/path/to/foo.go", 42, true),
			},
			fields: []zapcore.Field{
				zap.String("string", "test_\n_value"),
				zap.Strings("strings", []string{"\u001B1", "2\t"}),
				zap.Complex128p("complex", &[]complex128{12i - 8}[0]),
				zap.Int("int", -0),
				zap.Time("time", time.Date(2022, 6, 19, 16, 33, 42, 99, time.UTC)),
				zap.Duration("duration", 3*time.Hour),
				zap.Float64("float", -0.3e14),
			},
		},
		{
			desc: "Namespaces",
			// 4:33PM INF > test message test_string=test_message
			//  ↳ namespace.string2=val2 .string3=val3
			//             .namespace2.string4=val4 .string5=val5
			//                        .namespace3.namespace4.string6=val6 .string7=val7
			//                                              .namespace5
			expected: "\x1b[90m4:33PM\x1b[0m\x1b[32m \x1b[0m\x1b[32mINF\x1b[0m\x1b[32m \x1b[0m\x1b[1m\x1b[32m>\x1b[0m\x1b[0m\x1b[32m \x1b[0mtest message\x1b[32m \x1b[0m\x1b[32mtest_string=\x1b[0mtest_message\n\x1b[32m  ↳ namespace\x1b[0m\x1b[32m.string2=\x1b[0mval2\x1b[32m \x1b[0m\x1b[32m.string3=\x1b[0mval3\n             \x1b[32m.namespace2\x1b[0m\x1b[32m.string4=\x1b[0mval4\x1b[32m \x1b[0m\x1b[32m.string5=\x1b[0mval5\n                        \x1b[32m.namespace3\x1b[0m\x1b[32m.namespace4\x1b[0m\x1b[32m.string6=\x1b[0mval6\x1b[32m \x1b[0m\x1b[32m.string7=\x1b[0mval7\n                                              \x1b[32m.namespace5\x1b[0m\n",
			ent: zapcore.Entry{
				Level:   zapcore.InfoLevel,
				Message: "test message",
				Time:    time.Date(2018, 6, 19, 16, 33, 42, 99, time.UTC),
			},
			fields: []zapcore.Field{
				zap.String("test_string", "test_message"),
				zap.Namespace("namespace"),
				zap.String("string3", "val3"),
				zap.String("string2", "val2"),
				zap.Namespace("namespace2"),
				zap.String("string5", "val5"),
				zap.String("string4", "val4"),
				zap.Namespace("namespace3"),
				zap.Namespace("namespace4"),
				zap.String("string7", "val7"),
				zap.String("string6", "val6"),
				zap.Namespace("namespace5"),
			},
		},
		{
			desc: "Pre-formatted strings",
			// 4:33PM INF > test message test_string=test_message
			//   ↳ colours=RED STRING!
			//   ↳ namespace.mdb=db.users.find({
			// 						name: "James"
			// 				  });
			// 			 .sql=SELECT * FROM
			// 						users
			// 				  WHERE
			// 						name = 'James'
			expected: "\x1b[90m4:33PM\x1b[0m\x1b[32m \x1b[0m\x1b[32mINF\x1b[0m\x1b[32m \x1b[0m\x1b[1m\x1b[32m>\x1b[0m\x1b[0m\x1b[32m \x1b[0mtest message\x1b[32m \x1b[0m\x1b[32mtest_string=\x1b[0mtest_message\n\x1b[32m  ↳ colours\x1b[0m\x1b[32m=\x1b[0m\x1b[0m\x1b[31mRED STRING!\x1b[0m\x1b[31m\n\x1b[32m  ↳ namespace\x1b[0m\x1b[32m.mdb\x1b[0m\x1b[32m=\x1b[0mdb.users.find({\n                  \tname: \"James\"\n                  });\n             \x1b[32m.sql\x1b[0m\x1b[32m=\x1b[0mSELECT * FROM\n                  \tusers\n                  WHERE\n                  \tname = 'James'\n",
			ent: zapcore.Entry{
				Level:   zapcore.InfoLevel,
				Message: "test message",
				Time:    time.Date(2018, 6, 19, 16, 33, 42, 99, time.UTC),
			},
			fields: []zapcore.Field{
				zap.String("test_string", "test_message"),
				FormattedString("colours", "\x1b[0m\x1b[31mRED STRING!\x1b[0m\x1b[31m"),
				zap.Namespace("namespace"),
				FormattedString("sql", "SELECT * FROM\n\tusers\nWHERE\n\tname = 'James'"),
				zap.Any("mdb", FormattedStringValue("db.users.find({\n\tname: \"James\"\n});")),
			},
		},
		{
			desc: "Objects",
			// 4:33PM INF > test message
			//   ↳ object.1.1.1_leading_value=leading_value
			//               .2.1=string
			//                 .2=[1, 2, 3, 4]
			//                 .3=2.000000
			//                 .4.r1=[]string{
			//                         "r1", "r2", "r3", "r4", "r5", "r6", "r7", "r8",
			//                         "r9", "r10",
			//                       }
			//             .2=trailing_value
			expected: "\x1b[90m4:33PM\x1b[0m\x1b[32m \x1b[0m\x1b[32mINF\x1b[0m\x1b[32m \x1b[0m\x1b[1m\x1b[32m>\x1b[0m\x1b[0m\x1b[32m \x1b[0mtest message\n\x1b[32m  ↳ object\x1b[0m\x1b[32m.1\x1b[0m\x1b[32m.1\x1b[0m\x1b[32m.1_leading_value=\x1b[0mleading_value\n              \x1b[32m.2\x1b[0m\x1b[32m.1=\x1b[0mstring\n                \x1b[32m.2\x1b[0m\x1b[32m=[\x1b[0m1\x1b[32m, \x1b[0m2\x1b[32m, \x1b[0m3\x1b[32m, \x1b[0m4\x1b[32m]\x1b[0m\n                \x1b[32m.3\x1b[0m\x1b[32m=\x1b[0m2.000000\n                \x1b[32m.4\x1b[0m\x1b[32m.r1\x1b[0m\x1b[32m=\x1b[0m[]string{\n                        \"r1\", \"r2\", \"r3\", \"r4\", \"r5\", \"r6\", \"r7\", \"r8\",\n                        \"r9\", \"r10\",\n                      }\x1b[32m\n            \x1b[0m\x1b[32m.2=\x1b[0mtrailing_value\n",
			ent: zapcore.Entry{
				Level:   zapcore.InfoLevel,
				Message: "test message",
				Time:    time.Date(2018, 6, 19, 16, 33, 42, 99, time.UTC),
			},
			fields: []zapcore.Field{
				zap.Object("object", testStableMap{
					"1": testStableMap{
						"1": testStableMap{
							"1_leading_value": "leading_value",
							"2": testStableMap{
								"1": "string",
								"2": testArray{1, 2, 3, 4},
								"3": interface{}(2.0),
								"4": &testStableMap{"r1": []string{"r1", "r2", "r3", "r4", "r5", "r6", "r7", "r8", "r9", "r10"}},
							},
						},
						"2": "trailing_value",
					},
				}),
			},
		},
		{
			desc: "Arrays",
			// 4:33PM INF > test message
			//   ↳ array=[[1, 2, 3, 4],
			// 		      [],
			//		      [1, 2, 3,
			//			   [1]
			//		      ],
			//		      [1, 2, 3,
			//			   [{3=3 4=4}]
			//		      ],
			//		      [{1=1 2=2}, 3, 4, 5],
			//		      [1, 2,
			//			   {3=3 4=4}
			//		      ]
			//		     ]
			expected: "\x1b[90m4:33PM\x1b[0m\x1b[32m \x1b[0m\x1b[32mINF\x1b[0m\x1b[32m \x1b[0m\x1b[1m\x1b[32m>\x1b[0m\x1b[0m\x1b[32m \x1b[0mtest message\n\x1b[32m  ↳ array\x1b[0m\x1b[32m=[\x1b[0m\x1b[32m[\x1b[0m1\x1b[32m, \x1b[0m2\x1b[32m, \x1b[0m3\x1b[32m, \x1b[0m4\x1b[32m]\x1b[0m\x1b[32m, \x1b[0m\n           \x1b[32m[\x1b[0m\x1b[32m]\x1b[0m\x1b[32m, \x1b[0m\n           \x1b[32m[\x1b[0m1\x1b[32m, \x1b[0m2\x1b[32m, \x1b[0m3\x1b[32m, \x1b[0m\n            \x1b[32m[\x1b[0m1\x1b[32m]\x1b[0m\n           \x1b[32m]\x1b[0m\x1b[32m, \x1b[0m\n           \x1b[32m[\x1b[0m1\x1b[32m, \x1b[0m2\x1b[32m, \x1b[0m3\x1b[32m, \x1b[0m\n            \x1b[32m[\x1b[0m\x1b[32m{\x1b[0m\x1b[32m3=\x1b[0m3\x1b[32m \x1b[0m\x1b[32m4=\x1b[0m4\x1b[32m}\x1b[0m\x1b[32m]\x1b[0m\n           \x1b[32m]\x1b[0m\x1b[32m, \x1b[0m\n           \x1b[32m[\x1b[0m\x1b[32m{\x1b[0m\x1b[32m1=\x1b[0m1\x1b[32m \x1b[0m\x1b[32m2=\x1b[0m2\x1b[32m}\x1b[0m\x1b[32m, \x1b[0m3\x1b[32m, \x1b[0m4\x1b[32m, \x1b[0m5\x1b[32m]\x1b[0m\x1b[32m, \x1b[0m\n           \x1b[32m[\x1b[0m1\x1b[32m, \x1b[0m2\x1b[32m, \x1b[0m\n            \x1b[32m{\x1b[0m\x1b[32m3=\x1b[0m3\x1b[32m \x1b[0m\x1b[32m4=\x1b[0m4\x1b[32m}\x1b[0m\n           \x1b[32m]\x1b[0m\n          \x1b[32m]\x1b[0m\n",
			ent: zapcore.Entry{
				Level:   zapcore.InfoLevel,
				Message: "test message",
				Time:    time.Date(2018, 6, 19, 16, 33, 42, 99, time.UTC),
			},
			fields: []zapcore.Field{
				zap.Array("array", testArray{
					testArray{1, 2, 3, 4},
					testArray{},
					testArray{1, 2, 3, testArray{1}},
					testArray{1, 2, 3, testArray{&testStableMap{"3": 3, "4": 4}}},
					testArray{testStableMap{"1": 1, "2": 2}, 3, 4, 5},
					testArray{1, 2, testStableMap{"3": 3, "4": 4}},
				}),
			},
		},
		{
			desc: "Minimal Error",
			// 4:33PM ERR > test message
			//   ↳ error=Something \nwent wrong
			expected: "\x1b[90m4:33PM\x1b[0m\x1b[31m \x1b[0m\x1b[31mERR\x1b[0m\x1b[31m \x1b[0m\x1b[1m\x1b[31m>\x1b[0m\x1b[0m\x1b[31m \x1b[0mtest message\n\x1b[31m  ↳ error\x1b[0m\x1b[31m=\x1b[0mSomething \x1b[31m\\n\x1b[0mwent wrong\n",
			ent: zapcore.Entry{
				Level:   zapcore.ErrorLevel,
				Message: "test message",
				Time:    time.Date(2018, 6, 19, 16, 33, 42, 99, time.UTC),
			},
			fields: []zapcore.Field{
				zap.Error(errors.New("Something \nwent wrong")),
			},
		},
		{
			desc: "Errors",
			// 4:33PM ERR > test message named_stracktrace=<module_name>.TestEncodeEntry\n\t/<some_file>:<line_number>\ntesting.tRunner\n\t/<some_file>:<line_number>
			//  ↳ error=something \nwent wrong
			//  ↳ nested.cause=error with stacktrace
			//                 .cause.cause=error with 2 causes
			//                              .cause.cause.0=cause 1
			//                                             .stacktrace=github.com/thessem/zap-prettyconsole.TestEncodeEntry
			//                                                               /<some_file>:<line_number>
			//                                                         testing.tRunner
			//                                                               /<some_file>:<line_number>
			//                                                         runtime.goexit
			//                                                               /<some_file>:<line_number>
			//                                    .cause.1.cause=deeper error with two causes
			//                                                   .cause.cause.0=deeper cause 1
			//                                                         .cause.1=deeper cause 2
			//                                            .stacktrace=github.com/thessem/zap-prettyconsole.TestEncodeEntry
			//                                                               /<some_file>:<line_number>
			//                                                        testing.tRunner
			//                                                               /<some_file>:<line_number>
			//                                                        runtime.goexit
			//                                                               /<some_file>:<line_number>
			//                       .stacktrace=github.com/thessem/zap-prettyconsole.TestEncodeEntry
			//                                       /<some_file>:<line_number>
			//                                   testing.tRunner
			//                                       /<some_file>:<line_number>
			//                                   runtime.goexit
			//                                       /<some_file>:<line_number>
			//          .stacktrace=github.com/thessem/zap-prettyconsole.TestEncodeEntry
			//                       /<some_file>:<line_number>
			//                      testing.tRunner
			//                       /<some_file>:<line_number>
			//                      runtime.goexit
			//                       /<some_file>:<line_number>
			expected: "\x1b[90m4:33PM\x1b[0m\x1b[31m \x1b[0m\x1b[31mERR\x1b[0m\x1b[31m \x1b[0m\x1b[1m\x1b[31m>\x1b[0m\x1b[0m\x1b[31m \x1b[0mtest message\x1b[31m \x1b[0m\x1b[31mnamed_stracktrace=\x1b[0mgithub.com/thessem/zap-prettyconsole.TestEncodeEntry\x1b[31m\\n\x1b[0m\x1b[31m\\t\x1b[0m/<some_file>:<line_number>\x1b[31m\\n\x1b[0mtesting.tRunner\x1b[31m\\n\x1b[0m\x1b[31m\\t\x1b[0m/<some_file>:<line_number>\n\x1b[31m  ↳ error\x1b[0m\x1b[31m=\x1b[0msomething \x1b[31m\\n\x1b[0mwent wrong\n\x1b[31m  ↳ nested\x1b[0m\x1b[31m.cause\x1b[0m\x1b[31m=\x1b[0merror with stacktrace\n                 \x1b[31m.cause\x1b[0m\x1b[31m.cause\x1b[0m\x1b[31m=\x1b[0merror with 2 causes\n                              \x1b[31m.cause\x1b[0m\x1b[31m.cause.0\x1b[0m\x1b[31m=\x1b[0mcause 1\n                                             \x1b[31m.stacktrace=\x1b[0mgithub.com/thessem/zap-prettyconsole.TestEncodeEntry\n                                                         \t/<some_file>:<line_number>\n                                                         testing.tRunner\n                                                         \t/<some_file>:<line_number>\n                                                         runtime.goexit\n                                                         \t/<some_file>:<line_number>\n                                    \x1b[31m.cause.1\x1b[0m\x1b[31m.cause\x1b[0m\x1b[31m=\x1b[0mdeeper error with two causes\n                                                   \x1b[31m.cause\x1b[0m\x1b[31m.cause.0\x1b[0m\x1b[31m=\x1b[0mdeeper cause 1\n                                                         \x1b[31m.cause.1\x1b[0m\x1b[31m=\x1b[0mdeeper cause 2\n                                            \x1b[31m.stacktrace=\x1b[0mgithub.com/thessem/zap-prettyconsole.TestEncodeEntry\n                                                        \t/<some_file>:<line_number>\n                                                        testing.tRunner\n                                                        \t/<some_file>:<line_number>\n                                                        runtime.goexit\n                                                        \t/<some_file>:<line_number>\n                       \x1b[31m.stacktrace=\x1b[0mgithub.com/thessem/zap-prettyconsole.TestEncodeEntry\n                                   \t/<some_file>:<line_number>\n                                   testing.tRunner\n                                   \t/<some_file>:<line_number>\n                                   runtime.goexit\n                                   \t/<some_file>:<line_number>\n          \x1b[31m.stacktrace=\x1b[0mgithub.com/thessem/zap-prettyconsole.TestEncodeEntry\n                      \t/<some_file>:<line_number>\n                      testing.tRunner\n                      \t/<some_file>:<line_number>\n                      runtime.goexit\n                      \t/<some_file>:<line_number>\n\x1b[31m  ↳ nil_panic_PANIC_DISPLAYING_ERROR\x1b[0m\x1b[31m=\x1b[0mPANIC=runtime error: invalid memory address or nil pointer dereference\n\x1b[31m  ↳ normal_panic\x1b[0m<nil>\n\x1b[31m  ↳ stack\x1b[0m\x1b[31m=\x1b[0man error with a stacktrace has occurred\n          \x1b[31m.stacktrace=\x1b[0mgithub.com/thessem/zap-prettyconsole.TestEncodeEntry\n                      \t/<some_file>:<line_number>\n                      testing.tRunner\n                      \t/<some_file>:<line_number>\n                      runtime.goexit\n                      \t/<some_file>:<line_number>\n\x1b[31m  ↳ \x1b[0m\x1b[31mstacktrace=\x1b[0mgithub.com/thessem/zap-prettyconsole.TestEncodeEntry\n               \t/<some_file>:<line_number>\n               testing.tRunner\n               \t/<some_file>:<line_number>\n",
			ent: zapcore.Entry{
				Level:   zapcore.ErrorLevel,
				Message: "test message",
				Time:    time.Date(2018, 6, 19, 16, 33, 42, 99, time.UTC),
				Stack:   zap.Stack("ignored").String,
			},

			fields: []zapcore.Field{
				zap.Error(errors.New("something \nwent wrong")),
				zap.NamedError("stack", pkgerrors.New("an error with a stacktrace has occurred")),
				zap.NamedError("nested", pkgerrors.Wrap(
					pkgerrors.Wrapf(multierr.Combine(
						pkgerrors.New("cause 1"),
						pkgerrors.Wrapf(
							multierr.Combine(
								errors.New("deeper cause 1"),
								errors.New("deeper cause 2")),
							"deeper error with two causes"),
					), "error with 2 causes"),
					"error with stacktrace",
				)),
				zap.NamedError("nil_panic", (*testPanicError)(nil)),
				zap.NamedError("normal_panic", &[]testPanicError{"panic!"}[0]),
				zap.Stack("named_stracktrace"),
			},
		},

		{
			desc: "Go v1.20 Errors",
			// 4:33PM ERR > test message
			//  ↳ error=error with context
			//          .cause=cause 1
			//  ↳ error=errors with context
			//          .cause.0=cause 1
			//          .cause.1=cause 2
			//  ↳ error.cause.0=joined cause 1
			//         .cause.1=joined cause 2
			//  ↳ error=Joined and fmt
			//          .cause.0.cause.0=joined 1
			//                  .cause.1=joined 2
			//          .cause.1=fmt error
			//  ↳ nil_cause_error=Error has nil cause
			//  ↳ stacktrace=github.com/thessem/zap-prettyconsole.TestEncodeEntry
			//                       /<some_file>:<line_number>
			//               testing.tRunner
			//                       /<some_file>:<line_number>
			expected: "\x1b[90m4:33PM\x1b[0m\x1b[31m \x1b[0m\x1b[31mERR\x1b[0m\x1b[31m \x1b[0m\x1b[1m\x1b[31m>\x1b[0m\x1b[0m\x1b[31m \x1b[0mtest message\n\x1b[31m  ↳ error\x1b[0m\x1b[31m=\x1b[0merror with context\n          \x1b[31m.cause\x1b[0m\x1b[31m=\x1b[0mcause 1\n\x1b[31m  ↳ error\x1b[0m\x1b[31m=\x1b[0merrors with context\n          \x1b[31m.cause.0\x1b[0m\x1b[31m=\x1b[0mcause 1\n          \x1b[31m.cause.1\x1b[0m\x1b[31m=\x1b[0mcause 2\n\x1b[31m  ↳ error\x1b[0m\x1b[31m.cause.0\x1b[0m\x1b[31m=\x1b[0mjoined cause 1\n         \x1b[31m.cause.1\x1b[0m\x1b[31m=\x1b[0mjoined cause 2\n\x1b[31m  ↳ error\x1b[0m\x1b[31m=\x1b[0mJoined and fmt\n          \x1b[31m.cause.0\x1b[0m\x1b[31m.cause.0\x1b[0m\x1b[31m=\x1b[0mjoined 1\n                  \x1b[31m.cause.1\x1b[0m\x1b[31m=\x1b[0mjoined 2\n          \x1b[31m.cause.1\x1b[0m\x1b[31m=\x1b[0mfmt error\n\x1b[31m  ↳ nil_cause_error\x1b[0m\x1b[31m=\x1b[0mError has nil cause\n\x1b[31m  ↳ \x1b[0m\x1b[31mstacktrace=\x1b[0mgithub.com/thessem/zap-prettyconsole.TestEncodeEntry\n               \t/<some_file>:<line_number>\n               testing.tRunner\n               \t/<some_file>:<line_number>\n",
			ent: zapcore.Entry{
				Level:   zapcore.ErrorLevel,
				Message: "test message",
				Time:    time.Date(2018, 6, 19, 16, 33, 42, 99, time.UTC),
				Stack:   zap.Stack("ignored").String,
			},

			fields: []zapcore.Field{
				zap.Error(fmt.Errorf("error with context: %w", errors.New("cause 1"))),
				zap.Error(fmt.Errorf("errors with context: %w, %w", errors.New("cause 1"), errors.New("cause 2"))),
				zap.Error(errors.Join(errors.New("joined cause 1"), errors.New("joined cause 2"))),
				zap.Error(fmt.Errorf("Joined and fmt: %w and %w", errors.Join(fmt.Errorf("joined 1"), fmt.Errorf("joined 2")), fmt.Errorf("fmt error"))),
				zap.NamedError("nil_cause_error", nilCauseError{}),
				zap.NamedError("nill_error", nil),
			},
		},
	}

	enc := NewEncoder(NewEncoderConfig())

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			buf, err := enc.EncodeEntry(tt.ent, tt.fields)
			expected := rPath.ReplaceAllString(tt.expected, "/<some_file>:<line_number>")
			if assert.NoError(t, err, "Unexpected encoding error.") {
				log.Println(buf)
				got := rPath.ReplaceAllString(buf.String(), "/<some_file>:<line_number>")
				assert.Equalf(t, expected, got, "Incorrect encoded entry, received: \n%v", got)
			}
		})
	}
}

type testStableMap map[string]interface{}

func (t testStableMap) MarshalLogObject(encoder zapcore.ObjectEncoder) error {
	// Put these in alphabetical order so order doesn't change test-to-test
	keys := make([]string, 0, len(t))
	for k := range t {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		switch v := t[k].(type) {
		case zapcore.ObjectMarshaler:
			_ = encoder.AddObject(k, v)
		case zapcore.ArrayMarshaler:
			_ = encoder.AddArray(k, v)
		case string:
			encoder.AddString(k, v)
		case int:
			encoder.AddInt(k, v)
		default:
			_ = encoder.AddReflected(k, v)
		}
	}
	return nil
}

type testPanicError string

func (t *testPanicError) Error() string {
	if t == nil {
		return string(*t)
	} else {
		panic(*t)
	}
}

type nilCauseError struct{}

func (nilCauseError) Error() string {
	return "Error has nil cause"
}

func (nilCauseError) Cause() error {
	return nil
}

type testArray []interface{}

func (t testArray) MarshalLogArray(encoder zapcore.ArrayEncoder) error {
	for _, val := range t {
		switch v := val.(type) {
		case zapcore.ObjectMarshaler:
			_ = encoder.AppendObject(v)
		case zapcore.ArrayMarshaler:
			_ = encoder.AppendArray(v)
		case string:
			encoder.AppendString(v)
		case int:
			encoder.AppendInt(v)
		default:
			_ = encoder.AppendReflected(v)
		}
	}
	return nil
}

func TestIndentingWriter(t *testing.T) {
	tests := []struct {
		desc     string
		expected string
		input    string
	}{
		{
			desc:     "Empty",
			input:    "",
			expected: "",
		},
		{
			desc:     "No newlines",
			input:    "hello",
			expected: "hello",
		},
		{
			desc:     "Newlines",
			input:    "hello\nHow are\n\nYou?\n",
			expected: "hello\t\t  How are\t\t  \t\t  You?\t\t  ",
		},
		{
			desc:     "Trailing newline",
			input:    "T\n",
			expected: "T\t\t  ",
		},
		{
			desc:     "Leading newline",
			input:    "\nT",
			expected: "\t\t  T",
		},
		{
			desc:     "Only newline",
			input:    "\n",
			expected: "\t\t  ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			buf := buffer.Buffer{}
			iw := indentingWriter{indent: 2, buf: &buf, lineEnding: []byte{'\t', '\t'}}
			n, err := iw.Write([]byte(tt.input))
			assert.NoError(t, err)
			assert.Equal(t, buf.Len(), n)
			assert.Equal(t, tt.expected, buf.String())
		})
	}
}

func TestWith(t *testing.T) {
	cfg := NewEncoderConfig()
	cfg.TimeKey = zapcore.OmitKey
	enc := NewEncoder(cfg)
	buf := testBufferWriterSync{}
	pretty := zap.New(zapcore.NewCore(enc, &buf, zap.NewAtomicLevel()))

	// Regular With
	// WRN > wtf bark1=barv1 fook1=foov1
	pretty1 := pretty.With(zap.String("fook1", "foov1"))
	pretty1.Warn("wtf", zap.String("bark1", "barv1"))
	expected := "\x1b[33mWRN\x1b[0m\x1b[33m \x1b[0m\x1b[1m\x1b[33m>\x1b[0m\x1b[0m\x1b[33m \x1b[0mwtf\x1b[33m \x1b[0m\x1b[33mbark1=\x1b[0mbarv1\x1b[33m \x1b[0m\x1b[33mfook1=\x1b[0mfoov1\n"
	got := buf.buf.String()
	assert.Equalf(t, expected, got, "Incorrect encoded entry, recieved: \n%v", got)
	buf.buf.Reset()

	// Adding a namespace with With
	// WRN > wtf fook1=foov1
	//   ↳ fook11.bark11=barv11 .bark12=barv12
	pretty11 := pretty1.With(zap.Namespace("fook11"))
	pretty11 = pretty11.With(zap.String("bark12", "barv12"))
	pretty11.Warn("wtf", zap.String("bark11", "barv11"))
	expected = "\x1b[33mWRN\x1b[0m\x1b[33m \x1b[0m\x1b[1m\x1b[33m>\x1b[0m\x1b[0m\x1b[33m \x1b[0mwtf\x1b[33m \x1b[0m\x1b[33mfook1=\x1b[0mfoov1\n\x1b[33m  ↳ fook11\x1b[0m\x1b[33m.bark11=\x1b[0mbarv11\x1b[33m \x1b[0m\x1b[33m.bark12=\x1b[0mbarv12\n"
	got = buf.buf.String()
	assert.Equalf(t, expected, got, "Incorrect encoded entry, recieved: \n%v", got)
	buf.buf.Reset()

	// Making sure pretty didn't get modified above
	// WRN > wtf bark2=barv2 fook2=foov2
	pretty2 := pretty.With(zap.String("fook2", "foov2"))
	pretty2.Warn("wtf", zap.String("bark2", "barv2"))
	expected = "\x1b[33mWRN\x1b[0m\x1b[33m \x1b[0m\x1b[1m\x1b[33m>\x1b[0m\x1b[0m\x1b[33m \x1b[0mwtf\x1b[33m \x1b[0m\x1b[33mbark2=\x1b[0mbarv2\x1b[33m \x1b[0m\x1b[33mfook2=\x1b[0mfoov2\n"
	got = buf.buf.String()
	assert.Equalf(t, expected, got, "Incorrect encoded entry, recieved: \n%v", got)
	buf.buf.Reset()
}

type testBufferWriterSync struct {
	buf bytes.Buffer
}

func (w *testBufferWriterSync) Sync() error {
	return nil
}

func (w *testBufferWriterSync) Write(p []byte) (int, error) {
	return w.buf.Write(p)
}
