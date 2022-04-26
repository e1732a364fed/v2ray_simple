//go:build darwin || dragonfly || freebsd || linux || netbsd || openbsd
// +build darwin dragonfly freebsd linux netbsd openbsd

package utils

import (
	"io/ioutil"
	"math/rand"
	"os"
	"strings"
)

var words []string
var getWordListFailed bool

func GetRandomWord() (result string) {

	if len(words) == 0 && !getWordListFailed {
		words = readAvailableDictionary()
	}

	if theLen := len(words); theLen == 0 {
		getWordListFailed = true
		result = GenerateRandomString()
	} else {
		result = words[rand.Int()%theLen]
	}

	return
}

func readAvailableDictionary() (words []string) {
	file, err := os.Open("/usr/share/dict/words")
	if err != nil {
		return
	}

	bytes, err := ioutil.ReadAll(file)
	if err != nil {
		return
	}

	words = strings.Split(string(bytes), "\n")
	return
}
