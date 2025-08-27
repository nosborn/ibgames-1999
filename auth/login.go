package auth

import (
	"database/sql"
	"log"
	"math"
	"strings"

	"github.com/nosborn/ibgames-1999"
	"github.com/nosborn/ibgames-1999/db"
	"golang.org/x/crypto/bcrypt"
)

func Login(name, password, addr string) (*Session, LoginResult) {
	// Basic parameter sanity checking.
	if name == "" || password == "" {
		log.Print("Bad parameters to auth.Login")
		return nil, LoginError
	}

	// Remove leading and trailing whitespace from the name and password.
	name = strings.TrimSpace(name)
	if name == "" {
		log.Print("Bad parameters to auth.Login") // EXTRA
		return nil, LoginIncorrect
	}
	password = strings.TrimSpace(password)
	if password == "" {
		log.Print("Bad parameters to auth.Login") // EXTRA
		return nil, LoginIncorrect
	}

	// More parameter sanity checking.
	if len(name) > NameSize || len(password) > PasswordSize {
		log.Print("Bad parameters to auth.Login")
		return nil, LoginError
	}

	var (
		uid           ibgames.AccountID
		encrypt       string
		slogin        sql.NullString
		ulogin        sql.NullString
		sucip         sql.NullString
		nunsuclog     int
		unsucip       sql.NullString
		complimentary string
		status        string
		minutes       int
	)

	const query = `
		SELECT uid, encrypt, slogin, ulogin, sucip, nunsuclog, unsucip, complimentary, status, minutes
                FROM accounts
                WHERE name = ? COLLATE NOCASE`
	err := db.Tx().QueryRow(query, name).Scan(
		&uid, &encrypt, &slogin, &ulogin, &sucip, &nunsuclog, &unsucip, &complimentary, &status, &minutes)
	if err != nil {
		if err == sql.ErrNoRows {
			log.Print("NoRows") // EXTRA
			return nil, LoginIncorrect
		}
		log.Print(err) // EXTRA
		return nil, LoginError
	}

	switch status {
	case "A": // Active
	case "S": // Suspended - reject it later
	case "X": // Canceled
		return nil, LoginIncorrect
	default: /* Something else!? */
		log.Printf("status=%v", status) // EXTRA
		return nil, LoginError
	}

	//
	// dtcurrent(&now);
	// ip_address = inet_ntoa(addr);

	//
	if bcrypt.CompareHashAndPassword([]byte(encrypt), []byte(password)) != nil {
		log.Printf("Wrong password for %s", name)

		if nunsuclog < math.MaxInt16 {
			nunsuclog++
		}

		const query = `
			UPDATE accounts
			SET ulogin = CURRENT_TIMESTAMP, nunsuclog = ?, unsucip = ?
			WHERE uid = ?`
		result, err := db.Exec(query, nunsuclog, addr, uid)
		if err != nil {
			return nil, LoginError
		}
		if rows, err := result.RowsAffected(); err != nil || rows != 1 {
			return nil, LoginError
		}

		return nil, LoginIncorrect
	}

	// Now we can reject suspended accounts. This could be (== 'S') but (!=
	// 'A') is safer; anything other than Active or Suspended should have
	// been dealt with before here.
	if status != "A" {
		return nil, LoginSuspended
	}

	// If there have been too many unsuccessful password attempts then
	// we're not letting them in even though they got it right this time.
	// There's no need to update anything on the account record for this.
	if nunsuclog >= maxPasswordTries {
		log.Printf("Too many password failures for %s", name)
		return nil, LoginIncorrect
	}

	// Update the account to reflect a successful login.
	const updateStmt = `
		UPDATE accounts
		SET slogin = CURRENT_TIMESTAMP, sucip = ?, nunsuclog = 0
		WHERE uid = ?`
	result, err := db.Exec(updateStmt, addr, uid)
	if err != nil {
		return nil, LoginError
	}
	if rows, err := result.RowsAffected(); err != nil || rows != 1 {
		return nil, LoginError
	}

	// Pass back the session details.
	session := &Session{
		UID:     uid,
		SucIP:   sucip.String,
		UnsucIP: unsucip.String,
	}
	if slogin.Valid {
		session.SLogin = slogin.String
	} else {
		session.SLogin = "NEVER"
	}
	// else {
	//	-- reformat
	// }
	if ulogin.Valid {
		session.ULogin = ulogin.String
	} else {
		session.ULogin = "NEVER"
	}
	// else {
	//	-- reformat
	// }

	if complimentary != "Y" && minutes <= 0 {
		return session, LoginNoCredit
	}
	return session, LoginOK
}
