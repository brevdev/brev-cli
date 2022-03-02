package meshd

import (
	breverrors "github.com/brevdev/brev-cli/pkg/errors"

	"github.com/brevdev/brev-cli/pkg/terminal"
	"github.com/brevdev/brev-cli/pkg/vpn"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

type MeshDStore interface {
	vpn.ServiceMeshStore
}

func NewCmdMeshD(t *terminal.Terminal, store MeshDStore) *cobra.Command {
	cmd := &cobra.Command{
		Annotations:           map[string]string{"hidden": ""},
		Use:                   "meshd",
		DisableFlagsInUseLine: true,
		Short:                 "run background daemon for sercice mesh",
		Long:                  "run background daemon for sercice mesh",
		Run: func(cmd *cobra.Command, args []string) {
			err := MeshD(t, store)
			if err != nil {
				log.Error(err.Error())
			}
		},
	}

	return cmd
}

func MeshD(_ *terminal.Terminal, store MeshDStore) error {
	vpnd := &vpn.VPNDaemon{
		Store: store,
	}
	err := vpnd.Run()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	return nil
}
