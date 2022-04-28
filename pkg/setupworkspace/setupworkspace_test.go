package setupworkspace

import (
	"encoding/base64"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEvalAgent(_ *testing.T) {
	// cmd := CmdBuilder("bash", "/home/brev/workspace/brev-cli/test.sh")
	// c := "echo hi > /tmp/agent.sh"
	// cmd := exec.Command("bash", `-c`, c)
	// cmd.Stderr = os.Stderr
	// cmd.Stdout = os.Stdout
	// err := cmd.Run()
	// if err != nil {
	// 	panic(err)
	// }

	// t.Fail()
}

func TestFilePerm(_ *testing.T) {
	// f, err := os.Create("test")
	// if err != nil {
	// 	panic(err)
	// }
	// _, err = f.WriteString("blah\n")

	// if err != nil {
	// 	panic(err)
	// }
	// err = f.Chmod(0o400)
	// if err != nil {
	// 	panic(err)
	// }
}

func TestSendLogToFile(t *testing.T) {
	cmd := CmdBuilder("echo", "hi")
	done, err := SendLogToFile(cmd, "test.txt")
	assert.Nil(t, err)
	err = cmd.Run()
	assert.Nil(t, err)
	done()

	res, err := os.ReadFile("test.txt")
	assert.Nil(t, err)
	assert.Equal(t, "hi\n", string(res))
}

func Test_decodeBase64OrReturnSelf(t *testing.T) {
	nonB64 := "echo hi"
	resSelf := decodeBase64OrReturnSelf(nonB64)
	assert.Equal(t, nonB64, string(resSelf))
	b64 := base64.StdEncoding.EncodeToString([]byte(nonB64))
	res := decodeBase64OrReturnSelf(b64)
	assert.Equal(t, nonB64, string(res))
}
