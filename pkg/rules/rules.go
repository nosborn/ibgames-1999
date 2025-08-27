package rules

import (
	"fmt"

	"golang.org/x/sys/unix"

	"github.com/nosborn/ibgames-1999/pkg/goodies"
	"github.com/nosborn/ibgames-1999/pkg/ibgames"
)

var homeDir = goodies.HomeDir

func IsLockedOut(uid ibgames.AccountID) bool {
	if uid < ibgames.MinAccountID || uid > ibgames.MaxAccountID {
		panic(fmt.Sprintf("uid %d out of range [%d, %d]", uid, ibgames.MinAccountID, ibgames.MaxAccountID))
	}

	err := unix.Access(RulesLockFile(uid), unix.F_OK)
	return err == nil
}

func RulesLockFile(uid ibgames.AccountID) string {
	if uid < ibgames.MinAccountID || uid > ibgames.MaxAccountID {
		panic(fmt.Sprintf("uid %d out of range [%d, %d]", uid, ibgames.MinAccountID, ibgames.MaxAccountID))
	}

	return fmt.Sprintf("%s/lock/%d", homeDir(), uid)
}
