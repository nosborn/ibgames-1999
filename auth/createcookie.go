package auth

import (
	"net"

	"github.com/nosborn/ibgames-1999"
)

func CreateCookie(_ net.Addr, uid ibgames.AccountID, sid *string) CookieResult {
	// TODO: implementation
	return CookieOK
}
