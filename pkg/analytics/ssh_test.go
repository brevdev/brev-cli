package analytics

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

var (
	BasicStr   = "u_str                     ESTAB                     0                          0                                                                            * 5144862                                                      * 5144863                                                 "
	ProcessStr = "u_str                     ESTAB                     0                          0                                                  /run/systemd/journal/stdout 2179934                                                      * 2179933                                                 "
	URLStr     = "tcp                       ESTAB                     0                          0                                                                    127.0.0.1:ssh                                                  127.0.0.1:58670                                                   "
)

func Test_GetConnections(t *testing.T) {
	// needs ss to run
	c := NewConnLister()
	_, err := c.GetAllConnections()
	assert.Nil(t, err)
}

func Test_GetSSHConnections(t *testing.T) {
	c := ConnectionLister{
		connGetter: func() ([]byte, error) {
			res := strings.Join([]string{BasicStr, ProcessStr, URLStr}, "\n")
			return []byte(res), nil
		},
	}
	r, err := c.GetSSHConnections()
	assert.Nil(t, err)
	assert.Len(t, r, 1)
}

func Test_RowToStruct(t *testing.T) {
	res := RowStrToSSRow(URLStr)
	assert.Equal(t, res.LocalAddressPort, "127.0.0.1:ssh")

	res = RowStrToSSRow(ProcessStr)
	assert.Equal(t, res.LocalAddressPort, "/run/systemd/journal/stdout 2179934")

	res = RowStrToSSRow(BasicStr)
	assert.Equal(t, res.LocalAddressPort, "* 5144862")
}
