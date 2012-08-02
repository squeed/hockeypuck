package pq

import (
	_ "github.com/bmizerany/pq"
	"database/sql"
	"bytes"
	"crypto/rand"
	"encoding/ascii85"
	"errors"
	"io"
)

const UUID_LEN = 20

func NewUuid() (string, error) {
	buf := bytes.NewBuffer([]byte{})
	enc := ascii85.NewEncoder(buf)
	n, err := io.CopyN(enc, rand.Reader, UUID_LEN)
	if err != nil {
		return "", err
	}
	if n < UUID_LEN {
		return "", errors.New("Failed to generate UUID")
	}
	return string(buf.Bytes()), nil
}

type PqWorker struct {
	db *sql.DB
}

func NewWorker(connect string) (*PqWorker, error) {
	db, err := sql.Open("postgres", connect)
	if err != nil {
		return nil, err
	}
	return &PqWorker{ db: db }, nil
}

func (pq *PqWorker) GetKey(keyid string) (armor string, err error) {
	row := pq.db.QueryRow(`SELECT kl.armor
FROM pub_key pk JOIN key_log kl ON (pk.uuid = kl.pub_key_uuid)
WHERE creation < NOW() AND expiration > NOW()
AND state = 0
AND (pk.fingerprint = ? OR pk.long_id = ? OR pk.short_id = ?)
ORDER BY revision DESC
LIMIT 1`, keyid, keyid, keyid)
	if err != nil {
		return "", err
	}
	err = row.Scan(&armor)
	if err != nil {
		return "", err
	}
	return
}

func (pq *PqWorker) FindKeys(search string) (uuids []string, err error) {
	rows, err := pq.db.Query(`SELECT pub_key_uuid
FROM user_id
WHERE ts @@ to_tsquery(?)
AND creation < NOW() AND expiration > NOW()
AND state = 0
ORDER BY creation DESC
LIMIT 10`, search)
	uuids = []string{}
	var uuid string
	for rows.Next() {
		err = rows.Scan(&uuid)
		if err != nil {
			return
		}
		uuids = append(uuids, uuid)
	}
	return
}