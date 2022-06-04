package setupworkspace

import (
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
	done, err := SendLogToFiles(cmd, "test.txt")
	assert.Nil(t, err)
	err = cmd.Run()
	assert.Nil(t, err)
	done()

	res, err := os.ReadFile("test.txt")
	assert.Nil(t, err)
	assert.Equal(t, "hi\n", string(res))
}

func Test_AppendToOrCreateFile(t *testing.T) {
	_ = os.Remove("test2.txt")
	content := "hello"
	err := AppendToOrCreateFile("test2.txt", "hello")
	assert.Nil(t, err)
	res, err := os.ReadFile("test2.txt")
	assert.Nil(t, err)
	assert.Equal(t, content, string(res))
	err = AppendToOrCreateFile("test2.txt", "hello")
	assert.Nil(t, err)
	res, err = os.ReadFile("test2.txt")
	assert.Nil(t, err)
	assert.Equal(t, content+content, string(res))
}

func Test_getDefaultProjectFolderNameFromHost(t *testing.T) {
	res := getDefaultProjectFolderNameFromHost("test-working-dir-k13k-brevdev.wgt-us-west-2-test.brev.dev")
	assert.Equal(t, "test-working-dir", res)
	res = getDefaultProjectFolderNameFromHost("brevcli-zdud-brevdev.brev.sh")
	assert.Equal(t, "brevcli", res)
}
