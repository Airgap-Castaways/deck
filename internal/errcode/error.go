package errcode

import "fmt"

type Error struct {
	Code string
	Err  error
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	if e.Err == nil {
		return e.Code
	}
	if e.Code == "" {
		return e.Err.Error()
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Err.Error())
}

func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func New(code string, err error) error {
	if err == nil {
		return nil
	}
	return &Error{Code: code, Err: err}
}

func Newf(code string, format string, args ...any) error {
	return &Error{Code: code, Err: fmt.Errorf(format, args...)}
}

func Code(err error) string {
	var coded *Error
	if !As(err, &coded) || coded == nil {
		return ""
	}
	return coded.Code
}

func Is(err error, code string) bool {
	return Code(err) == code
}

func As(err error, target **Error) bool {
	if err == nil {
		return false
	}
	for err != nil {
		coded, ok := err.(*Error)
		if ok {
			*target = coded
			return true
		}
		unwrapper, ok := err.(interface{ Unwrap() error })
		if !ok {
			return false
		}
		err = unwrapper.Unwrap()
	}
	return false
}
