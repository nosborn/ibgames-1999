package db

import "github.com/nosborn/ibgames-1999/pkg/ibgames"

func IsCancelled(uid ibgames.AccountID) (bool, error) {
	var status string
	err := tx.QueryRow("SELECT json_extract(data, '$.status') FROM accounts WHERE uid = ?", uid).Scan(&status)
	if err != nil {
		return false, err
	}
	return (status == "X"), nil
}
