//go:build !linux

package disablessh

import (
	"context"
	"fmt"
)

func listLocalAccounts(context.Context) ([]localAccount, error) {
	return nil, fmt.Errorf("brev disable-ssh local cleanup is only supported on Linux")
}

func cleanLocalAccount(localAccount) (int, error) {
	return 0, fmt.Errorf("brev disable-ssh local cleanup is only supported on Linux")
}
