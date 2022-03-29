package utils

import (
	"bytes"
	"fmt"
	"strconv"
)

//没啥特殊的
type NumErr struct {
	N      int
	Prefix string
}

func (ne *NumErr) Error() string {

	return ne.Prefix + strconv.Itoa(ne.N)
}

//就是带个buffer的普通ErrInErr，没啥特殊的
type ErrFirstBuffer struct {
	Err   error
	First *bytes.Buffer
}

func (ef *ErrFirstBuffer) Unwarp() error {

	return ef.Err
}

func (ef *ErrFirstBuffer) Error() string {

	return ef.Err.Error()
}

func NewErr(desc string, e error) *ErrInErr {
	return &ErrInErr{
		ErrDesc:   desc,
		ErrDetail: e,
	}
}

func NewDataErr(desc string, e error, data interface{}) *ErrInErr {
	return &ErrInErr{
		ErrDesc:   desc,
		ErrDetail: e,
		Data:      data,
	}
}

// ErrInErr 很适合一个err包含另一个err，并且提供附带数据的情况
type ErrInErr struct {
	ErrDesc   string
	ErrDetail error
	Data      any

	cachedStr string
}

func (e *ErrInErr) Error() string {
	return e.String()
}

func (e *ErrInErr) Unwarp() error {

	return e.ErrDetail
}

func (e *ErrInErr) String() string {
	if e.cachedStr == "" {
		e.cachedStr = e.string()
	}
	return e.cachedStr
}

func (e *ErrInErr) string() string {
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
