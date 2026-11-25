package secrets

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"

	"golang.org/x/crypto/argon2"
)

const (
	timeCost uint32 = 2
	memory   uint32 = 64 * 1024
	threads  uint8  = 2
	keyLen   uint32 = 32
)

func HashPassword(password string) (string, error) {
	if len(password) < 12 {
		return "", fmt.Errorf("password must contain at least 12 bytes")
	}
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	hash := argon2.IDKey([]byte(password), salt, timeCost, memory, threads, keyLen)
	enc := base64.RawStdEncoding
	return fmt.Sprintf("$argon2id$v=19$m=%d,t=%d,p=%d$%s$%s", memory, timeCost, threads, enc.EncodeToString(salt), enc.EncodeToString(hash)), nil
}

func VerifyPassword(encoded, password string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" || parts[2] != "v=19" {
		return false
	}
	params := map[string]uint64{}
	for _, p := range strings.Split(parts[3], ",") {
		kv := strings.SplitN(p, "=", 2)
		if len(kv) != 2 {
			return false
		}
		n, e := strconv.ParseUint(kv[1], 10, 32)
		if e != nil {
			return false
		}
		params[kv[0]] = n
	}
	if params["m"] < 8*1024 || params["m"] > 256*1024 || params["t"] < 1 || params["t"] > 10 || params["p"] < 1 || params["p"] > 16 {
		return false
	}
	enc := base64.RawStdEncoding
	salt, e := enc.DecodeString(parts[4])
	if e != nil || len(salt) < 16 || len(salt) > 64 {
		return false
	}
	want, e := enc.DecodeString(parts[5])
	if e != nil || len(want) < 16 || len(want) > 64 {
		return false
	}
	got := argon2.IDKey([]byte(password), salt, uint32(params["t"]), uint32(params["m"]), uint8(params["p"]), uint32(len(want)))
	return subtle.ConstantTimeCompare(got, want) == 1
}

type Secret string

func (Secret) String() string   { return "[REDACTED]" }
func (Secret) GoString() string { return "[REDACTED]" }
