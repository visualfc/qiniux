/*
 Copyright 2022 Qiniu Limited (qiniu.com)

 Licensed under the Apache License, Version 2.0 (the "License");
 you may not use this file except in compliance with the License.
 You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

 Unless required by applicable law or agreed to in writing, software
 distributed under the License is distributed on an "AS IS" BASIS,
 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 See the License for the specific language governing permissions and
 limitations under the License.
*/

// Package errors provides errors stack tracking utilities.
package errors

import (
	"errors"
	"fmt"
	"io"
	"reflect"
	"runtime"
	"strconv"
	"strings"
)

// --------------------------------------------------------------------

// New returns an error that formats as the given text.
// Each call to New returns a distinct error value even if the text is identical.
func New(msg string) error {
	return errors.New(msg)
}

// Err returns the cause error.
func Err(err error) error {
	if e, ok := err.(*Frame); ok {
		return Err(e.Err)
	}
	return err
}

// Summary returns summary of specified error.
func Summary(err error) string {
	e, ok := err.(interface {
		Summary() string
	})
	if !ok {
		return err.Error()
	}
	return e.Summary()
}

// --------------------------------------------------------------------

// NotFound is a generic NotFound error.
type NotFound struct {
	Category string
}

func (p *NotFound) Error() string {
	return p.Category + " not found"
}

// IsNotFound unwraps err and checks it is a *NotFound object or not.
func IsNotFound(err error) bool {
	for {
		if e, ok := err.(interface{ Unwrap() error }); ok {
			err = e.Unwrap()
		} else {
			_, ok = err.(*NotFound)
			return ok
		}
	}
}

// --------------------------------------------------------------------

type List []error

func (p *List) Add(err error) {
	*p = append(*p, err)
}

func (p List) Error() string {
	n := len(p)
	if n >= 2 {
		s := make([]string, n)
		for i, v := range p {
			s[i] = v.Error()
		}
		return strings.Join(s, "\n")
	}
	if n == 1 {
		return p[0].Error()
	}
	return ""
}

func (p List) Summary() string {
	n := len(p)
	if n >= 2 {
		s := make([]string, n)
		for i, v := range p {
			s[i] = Summary(v)
		}
		return strings.Join(s, "\n")
	}
	if n == 1 {
		return Summary(p[0])
	}
	return ""
}

func (p List) ToError() error {
	switch len(p) {
	case 1:
		return p[0]
	case 0:
		return nil
	}
	return p
}

// Format is required by fmt.Formatter
func (p List) Format(s fmt.State, verb rune) {
	switch verb {
	case 'v':
		io.WriteString(s, p.Error())
	case 's':
		io.WriteString(s, p.Summary())
	case 'q':
		fmt.Fprintf(s, "%q", p.Error())
	}
}

// --------------------------------------------------------------------

// Frame represents an error frame.
type Frame struct {
	Err  error
	Func string
	Args []interface{}
	Code string
	File string
	Line int
}

// NewWith creates a new error frame.
func NewWith(err error, code string, n int, fn string, args ...interface{}) *Frame {
	file, line := fileLine()
	return &Frame{Err: err, Func: fn, Args: args, Code: code, File: file, Line: line + n}
}

func fileLine() (file string, line int) {
	_, file, line, _ = runtime.Caller(2)
	return
}

// NewFrame creates a new error frame.
func NewFrame(err error, code, file string, line int, fn string, args ...interface{}) *Frame {
	return &Frame{Err: err, Func: fn, Args: args, Code: code, File: file, Line: line}
}

func (p *Frame) Error() string {
	return string(errorDetail(make([]byte, 0, 32), p))
}

func (p *Frame) Summary() string {
	return Summary(p.Err)
}

func errorDetail(b []byte, p *Frame) []byte {
	if f, ok := p.Err.(*Frame); ok {
		b = errorDetail(b, f)
	} else {
		b = append(b, p.Err.Error()...)
		b = append(b, "\n\n===> errors stack:\n"...)
	}
	b = append(b, p.Func...)
	b = append(b, '(')
	b = argsDetail(b, p.Args)
	b = append(b, ")\n\t"...)
	b = append(b, p.File...)
	b = append(b, ':')
	b = strconv.AppendInt(b, int64(p.Line), 10)
	b = append(b, ' ')
	b = append(b, p.Code...)
	b = append(b, '\n')
	return b
}

func argsDetail(b []byte, args []interface{}) []byte {
	nlast := len(args) - 1
	for i, arg := range args {
		b = appendValue(b, arg)
		if i != nlast {
			b = append(b, ',', ' ')
		}
	}
	return b
}

func appendValue(b []byte, arg interface{}) []byte {
	if arg == nil {
		return append(b, "nil"...)
	}
	v := reflect.ValueOf(arg)
	kind := v.Kind()
	if kind >= reflect.Bool && kind <= reflect.Complex128 {
		return append(b, fmt.Sprint(arg)...)
	}
	if kind == reflect.String {
		val := arg.(string)
		if len(val) > 32 {
			val = val[:16] + "..." + val[len(val)-16:]
		}
		return strconv.AppendQuote(b, val)
	}
	if kind == reflect.Array {
		return append(b, "Array"...)
	}
	if kind == reflect.Struct {
		return append(b, "Struct"...)
	}
	val := v.Pointer()
	b = append(b, '0', 'x')
	return strconv.AppendInt(b, int64(val), 16)
}

// Unwrap provides compatibility for Go 1.13 error chains.
func (p *Frame) Unwrap() error {
	return p.Err
}

// Format is required by fmt.Formatter
func (p *Frame) Format(s fmt.State, verb rune) {
	switch verb {
	case 'v':
		io.WriteString(s, p.Error())
	case 's':
		io.WriteString(s, p.Summary())
	case 'q':
		fmt.Fprintf(s, "%q", p.Error())
	}
}

// --------------------------------------------------------------------

// CallDetail print a function call shortly.
func CallDetail(msg []byte, fn interface{}, args ...interface{}) []byte {
	f := runtime.FuncForPC(reflect.ValueOf(fn).Pointer())
	if f != nil {
		msg = append(msg, f.Name()...)
		msg = append(msg, '(')
		msg = argsDetail(msg, args)
		msg = append(msg, ')')
	}
	return msg
}

// --------------------------------------------------------------------

// ErrorInfo is provided for backward compatibility
//
// Deprecated: Use Frame instead.
type ErrorInfo = Frame

// Detail is provided for backward compatibility
func (p *ErrorInfo) Detail(err error) *ErrorInfo {
	p.Code = err.Error()
	return p
}

// NestedObject is provided for backward compatibility
func (p *ErrorInfo) NestedObject() interface{} {
	return p.Err
}

// ErrorDetail is provided for backward compatibility
func (p *ErrorInfo) ErrorDetail() string {
	return p.Error()
}

// AppendErrorDetail is provided for backward compatibility
func (p *ErrorInfo) AppendErrorDetail(b []byte) []byte {
	return errorDetail(b, p)
}

// SummaryErr is provided for backward compatibility
func (p *ErrorInfo) SummaryErr() error {
	return p.Err
}

// Info is provided for backward compatibility
//
// Deprecated: Use NewWith instead.
func Info(err error, cmd ...interface{}) *ErrorInfo {
	return &Frame{Err: err, Args: cmd}
}

// InfoEx is provided for backward compatibility
//
// Deprecated: Use NewWith instead.
func InfoEx(calldepth int, err error, cmd ...interface{}) *ErrorInfo {
	return &Frame{Err: err, Args: cmd}
}

// Detail is provided for backward compatibility
//
// Deprecated: Use err.Error() instead.
func Detail(err error) string {
	return err.Error()
}

// --------------------------------------------------------------------
