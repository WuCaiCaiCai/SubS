package backup

import (
	"bytes"
	"testing"
)

func TestBuildReadRoundTrip(t *testing.T) {
	files := map[string][]byte{
		"metadata.json": []byte(`{"format":"subs.sbox","version":1}`),
		"db.sqlite":     []byte("SQLite format 3\x00 fake"),
	}
	archive, err := Build(files, "correct horse battery staple")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(archive, []byte("age-encryption.org/")) {
		t.Fatalf("archive is not age encrypted: %q", archive[:24])
	}
	got, err := Read(archive, "correct horse battery staple")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got["db.sqlite"], files["db.sqlite"]) {
		t.Error("db payload mismatch")
	}
	if !bytes.Equal(got["metadata.json"], files["metadata.json"]) {
		t.Error("metadata payload mismatch")
	}
}

func TestReadWrongPassword(t *testing.T) {
	archive, err := Build(map[string][]byte{"db.sqlite": []byte("x")}, "correct horse battery staple")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Read(archive, "wrong password here"); err == nil {
		t.Fatal("expected decryption failure")
	}
}

func TestReadMissingDatabase(t *testing.T) {
	archive, err := Build(map[string][]byte{"metadata.json": []byte("{}")}, "correct horse battery staple")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Read(archive, "correct horse battery staple"); err == nil {
		t.Fatal("expected missing database error")
	}
}
