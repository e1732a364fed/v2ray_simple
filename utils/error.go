package utils

import (
	"bytes"
	"errors"
	"fmt"
	"strconv"
)

var (
	ErrNotImplemented      = errors.New("not implemented")
	ErrNilParameter        = errors.New("nil parameter")
	ErrNilOrWrongParameter = errors.New("nil or wrong parameter")
	ErrWrongParameter      = errors.New("wrong parameter")
	ErrInvalidData         = errors.New("invalid data")

	ErrShortRead = errors.New("short read")
	ErrHandled   = errors.New("handled")
	ErrFailed    = errors.New("failed") //最无脑的Err, 在能描述清楚错误时不要使用 ErrFailed
)

//nothing special
type NumErr struct {
	N      int
	Prefix string
}

func (ne NumErr) Error() string {

	return ne.Prefix + strconv.Itoa(ne.N)
}

//an err with a buffer, nothing special
type ErrBuffer struct {
	Err error
	Buf *bytes.Buffer
}

func (ef ErrBuffer) Unwarp() error {

	return ef.Err
}

func (ef ErrBuffer) Error() string {

	return ef.Err.Error() + ", with Buffer."
}

// ErrInErr 很适合一个err包含另一个err，并且提供附带数据的情况.
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

func (e ErrInErr) Is(err error) bool {
	if e.ErrDetail == err {
		return true
	} else if errors.Is(e.ErrDetail, err) {
		return true
	} else if len(e.ExtraIs) > 0 {
		for _, v := range e.ExtraIs {
			if errors.Is(v, err) {
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
