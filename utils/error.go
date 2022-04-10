package utils

import (
	"bytes"
	"errors"
	"fmt"
	"strconv"
)

var ErrNotImplemented = errors.New("not implemented")
var ErrNilParameter = errors.New("nil parameter")
var ErrNilOrWrongParameter = errors.New("nil or wrong parameter")
var ErrWrongParameter = errors.New("wrong parameter")
var ErrShortRead = errors.New("short read")
var ErrInvalidData = errors.New("invalid data")

//没啥特殊的
type NumErr struct {
	N      int
	Prefix string
}

func (ne NumErr) Error() string {

	return ne.Prefix + strconv.Itoa(ne.N)
}

//就是带个buffer的普通ErrInErr，没啥特殊的
type ErrFirstBuffer struct {
	Err   error
	First *bytes.Buffer
}

func (ef ErrFirstBuffer) Unwarp() error {

	return ef.Err
}

func (ef ErrFirstBuffer) Error() string {

	return ef.Err.Error()
}

// 返回结构体，而不是指针, 这样可以避免内存逃逸到堆
// 发现只要是函数就会逃逸到堆，自己初始化就没事。那就不提供初始化函数了。
/*func NewErr(desc string, e error) ErrInErr {
	return ErrInErr{
		ErrDesc:   desc,
		ErrDetail: e,
	}
}

// 返回结构体，而不是指针, 这样可以避免内存逃逸到堆

func NewDataErr(desc string, e error, data interface{}) ErrInErr {
	return ErrInErr{
		ErrDesc:   desc,
		ErrDetail: e,
		Data:      data,
	}
}
*/

// ErrInErr 很适合一个err包含另一个err，并且提供附带数据的情况.
type ErrInErr struct {
	ErrDesc   string
	ErrDetail error
	Data      any
}

func (e ErrInErr) Error() string {
	return e.String()
}

func (e ErrInErr) Unwarp() error {

	return e.ErrDetail
}

func (e ErrInErr) Is(err error) bool {
	return e.ErrDetail == err
}

func (e ErrInErr) String() string {

	if e.Data != nil {

		if e.ErrDetail != nil {
			return fmt.Sprintf("%s : %s, Data: %v", e.ErrDesc, e.ErrDetail.Error(), e.Data)

		}

		return fmt.Sprintf("%s , Data: %v", e.ErrDesc, e.Data)

	}
	if e.ErrDetail != nil {
		return fmt.Sprintf("%s : %s", e.ErrDesc, e.ErrDetail.Error())

	}
	return e.ErrDesc

}
