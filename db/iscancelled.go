package db

import "github.com/nosborn/ibgames-1999"

func IsCancelled(uid ibgames.AccountID) (bool, error) {
	var status string
	err := tx.QueryRow("SELECT status FROM accounts WHERE uid = ?", uid).Scan(&status)
	if err != nil {
		return false, err
	}
	return (status == "X"), nil
}
