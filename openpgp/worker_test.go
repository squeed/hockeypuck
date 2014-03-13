/*
   Hockeypuck - OpenPGP key server
   Copyright (C) 2012-2014  Casey Marshall

   This program is free software: you can redistribute it and/or modify
   it under the terms of the GNU Affero General Public License as published by
   the Free Software Foundation, version 3.

   This program is distributed in the hope that it will be useful,
   but WITHOUT ANY WARRANTY; without even the implied warranty of
   MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
   GNU Affero General Public License for more details.

   You should have received a copy of the GNU Affero General Public License
   along with this program.  If not, see <http://www.gnu.org/licenses/>.
*/

package openpgp

import (
	"crypto/md5"
	"fmt"
	"testing"

	"code.google.com/p/go.crypto/openpgp/armor"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"

	"github.com/squeed/hockeypuck"
)

func connectString() string {
	return fmt.Sprintf(
		"dbname=postgres host=/var/run/postgresql sslmode=disable user=%s", currentUsername())
}

func MustCreateWorker(t *testing.T) *Worker {
	db, err := sqlx.Connect("postgres", connectString())
	assert.Nil(t, err)
	db.Execf("DROP DATABASE IF EXISTS testhkp")
	db.Execf("CREATE DATABASE testhkp")
	hockeypuck.SetConfig(fmt.Sprintf(`
[hockeypuck.openpgp.db]
driver="postgres"
dsn="dbname=testhkp host=/var/run/postgresql sslmode=disable user=%s"
`, currentUsername()))
	w, err := NewWorker(nil, nil)
	assert.Nil(t, err)
	return w
}

func MustDestroyWorker(t *testing.T, w *Worker) {
	w.db.Close()
	db, err := sqlx.Connect("postgres", connectString())
	assert.Nil(t, err)
	db.Close()
}

func TestValidateKey(t *testing.T) {
	f := MustInput(t, "tails.asc")
	defer f.Close()
	block, err := armor.Decode(f)
	if err != nil {
		t.Fatal(err)
	}
	var keys []*Pubkey
	for keyRead := range ReadKeys(block.Body) {
		keys = append(keys, keyRead.Pubkey)
	}
	assert.Equal(t, 1, len(keys))
	assert.Equal(t, 2, len(keys[0].userIds))
	for i := 0; i < 2; i++ {
		assert.NotEmpty(t, keys[0].userIds[i].ScopedDigest)
	}
}

func TestRoundTripKeys(t *testing.T) {
	for _, testfile := range []string{
		"sksdigest.asc", "alice_signed.asc", "alice_unsigned.asc",
		"uat.asc", "tails.asc", "fece664e.asc", "weasel.asc"} {
		testRoundTripKey(t, testfile)
	}
}

func testRoundTripKey(t *testing.T, testfile string) {
	w := MustCreateWorker(t)
	defer MustDestroyWorker(t, w)
	key1 := MustInputAscKey(t, testfile)
	Resolve(key1)
	_, err := w.Begin()
	assert.Nil(t, err)
	keyChange := w.UpsertKey(key1)
	assert.Nil(t, keyChange.Error)
	err = w.Commit()
	assert.Nil(t, err)
	key2, err := w.fetchKey(key1.RFingerprint)
	if err != nil {
		t.Fatal(err)
	}
	h1 := SksDigest(key1, md5.New())
	h2 := SksDigest(key2, md5.New())
	assert.Equal(t, h1, h2)
	assert.Equal(t, key1.Md5, h1)
	assert.Equal(t, key2.Md5, h1)
}
