// Package backup packs and unpacks .sbox archives: an age-encrypted tar.gz of
// metadata.json plus a VACUUM INTO snapshot of the SQLite database.
package backup

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"errors"
	"io"
	"sort"
	"time"

	"filippo.io/age"
)

// MaxSize caps each unpacked member at 512 MiB.
const MaxSize = 512 << 20

// Build creates an age-scrypt encrypted tar.gz of the given files.
func Build(files map[string][]byte, password string) ([]byte, error) {
	// tar members are written in a stable order so identical inputs produce
	// identical plaintext (helps tests and reproducible debugging).
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)

	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for _, name := range names {
		data := files[name]
		hdr := &tar.Header{Name: name, Mode: 0o600, Size: int64(len(data)), ModTime: time.Now()}
		if err := tw.WriteHeader(hdr); err != nil {
			return nil, err
		}
		if _, err := tw.Write(data); err != nil {
			return nil, err
		}
	}
	if err := tw.Close(); err != nil {
		return nil, err
	}
	if err := gz.Close(); err != nil {
		return nil, err
	}
	recipient, err := age.NewScryptRecipient(password)
	if err != nil {
		return nil, err
	}
	var out bytes.Buffer
	aw, err := age.Encrypt(&out, recipient)
	if err != nil {
		return nil, err
	}
	if _, err := aw.Write(buf.Bytes()); err != nil {
		return nil, err
	}
	if err := aw.Close(); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

// Read decrypts and unpacks an .sbox archive, ignoring unknown members.
func Read(data []byte, password string) (map[string][]byte, error) {
	identity, err := age.NewScryptIdentity(password)
	if err != nil {
		return nil, errors.New("备份密码无效")
	}
	ar, err := age.Decrypt(bytes.NewReader(data), identity)
	if err != nil {
		return nil, errors.New("备份解密失败：密码错误或文件损坏")
	}
	gz, err := gzip.NewReader(ar)
	if err != nil {
		return nil, errors.New("备份格式错误")
	}
	tr := tar.NewReader(gz)
	out := map[string][]byte{}
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, errors.New("备份格式错误")
		}
		if hdr.Name != "metadata.json" && hdr.Name != "db.sqlite" {
			continue
		}
		if hdr.Size > MaxSize {
			return nil, errors.New("备份文件过大")
		}
		body, err := io.ReadAll(io.LimitReader(tr, MaxSize))
		if err != nil {
			return nil, errors.New("读取备份失败")
		}
		out[hdr.Name] = body
	}
	if out["db.sqlite"] == nil {
		return nil, errors.New("备份缺少数据库")
	}
	return out, nil
}
