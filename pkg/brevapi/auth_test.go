package brevapi

// Basic imports
import (
	"github.com/brevdev/brev-cli/pkg/store"
	"github.com/spf13/afero"

	"github.com/stretchr/testify/suite"
)

type BrevAPITestSuite struct {
	suite.Suite
	BrevAPIFs store.FileStore
}

func (suite *BrevSSHTestSuite) SetupTest() {
	mmfs := afero.NewMemMapFs()
	s := store.NewBasicStore().WithFileSystem(mmfs)
	s.cre
}
