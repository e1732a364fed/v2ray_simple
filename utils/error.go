package utils

import (
	"bytes"
	"errors"
	"fmt"
	"strconv"
)

var (
	ErrUnImplemented       = errors.New("not implemented")
	ErrNilParameter        = errors.New("nil parameter")
	ErrNilOrWrongParameter = errors.New("nil or wrong parameter")
	ErrWrongParameter      = errors.New("wrong parameter")
	ErrInvalidData         = errors.New("invalid data")
	ErrNoMatch             = errors.New("no matched")
	ErrInvalidNumber       = errors.New("invalid number")

	ErrShortRead = errors.New("short read")
	ErrHandled   = errors.New("handled")
	ErrFailed    = errors.New("failed") //最无脑的Err, 在能描述清楚错误时不要使用 ErrFailed
)

type InvalidDataErr string

//return err == e || err == ErrInvalidData
func (e InvalidDataErr) Is(err error) bool {
	return err == e || err == ErrInvalidData
}

func (e InvalidDataErr) Error() string {
	return string(e)
}

//nothing special. Normally, N==0 means no error
type NumErr struct {
	N int
	E error
}

func (ne NumErr) Error() string {

	return ne.E.Error() + ", " + strconv.Itoa(ne.N)
}

func (e NumErr) Is(target error) bool {
	return errors.Is(e.E, target)

}
func (ef NumErr) Unwarp() error {

	return ef.E
}

//nothing special
type NumStrErr struct {
	N      int
	Prefix string
}

func (ne NumStrErr) Error() string {

	return ne.Prefix + strconv.Itoa(ne.N)
}

//an err with a buffer, nothing special
type ErrBuffer struct {
	Err error
	Buf *bytes.Buffer
}

func (e ErrBuffer) Is(target error) bool {
	return errors.Is(e.Err, target)

}

func (ef ErrBuffer) Unwarp() error {

	return ef.Err
}

func (ef ErrBuffer) Error() string {

	if ef.Buf != nil {
		return ef.Err.Error() + ", with Buffer,len " + strconv.Itoa(ef.Buf.Len())

	} else {
		return ef.Err.Error() + ", with nil Buffer."

	}
}

// ErrInErr 很适合一个err包含另一个err，并且提供附带数据的情况. 类似 fmt.Errorf.
type ErrInErr struct {
	ErrDesc   string
	ErrDetail error
	Data      any

	ExtraIs []error
}

func (e ErrInErr) Error() string {
	return e.String()
}

func (e ErrInErr) Unwarp() error {
	return e.ErrDetail
}

func (e ErrInErr) Is(target error) bool {
	if e.ErrDetail == target {
		return true
	} else if errors.Is(e.ErrDetail, target) {
		return true
	} else if len(e.ExtraIs) > 0 {
		for _, v := range e.ExtraIs {
			if errors.Is(v, target) {
				return true
			}
		}
	}
	return false
}

func (e ErrInErr) String() string {

	if e.Data != nil {

		if e.ErrDetail != nil {
			return fmt.Sprintf(" [ %s , Detail: %s, Data: %v ] ", e.ErrDesc, e.ErrDetail.Error(), e.Data)

		}

		return fmt.Sprintf(" [ %s , Data: %v ] ", e.ErrDesc, e.Data)
	}

	if e.ErrDetail != nil {
		return fmt.Sprintf(" [ %s , Detail: %s ] ", e.ErrDesc, e.ErrDetail.Error())
	}

	return e.ErrDesc

}
