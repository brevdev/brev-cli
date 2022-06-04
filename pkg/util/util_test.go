package util

import (
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_decodeBase64OrReturnSelf(t *testing.T) {
	nonB64 := "echo hi"
	resSelf := DecodeBase64OrReturnSelf(nonB64)
	assert.Equal(t, nonB64, string(resSelf))
	b64 := base64.StdEncoding.EncodeToString([]byte(nonB64))
	res := DecodeBase64OrReturnSelf(b64)
	assert.Equal(t, nonB64, string(res))
}

func TestBasePath(t *testing.T) {
	x := "abc/setup.log"
	b := RemoveFileExtenstion(x)
	assert.Equal(t, "abc/setup", b)
}
