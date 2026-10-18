package sandbox

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"github.com/Calcium-Ion/moejs"
)

// cryptoFunctions provides UTF-8 hashing and hexadecimal digests for API request signing.
func cryptoFunctions() map[string]any {
	return map[string]any{
		"sha256": moejs.NativeFunc(func(_ *moejs.Realm, _ moejs.Value, args []moejs.Value) (moejs.Value, error) {
			data := moejs.Arg(args, 0)
			if !data.IsString() {
				return moejs.Undefined(), fmt.Errorf("crypto.sha256: data must be a string")
			}

			digest := sha256.Sum256([]byte(data.String()))
			return moejs.String(hex.EncodeToString(digest[:])), nil
		}),

		"hmacSha256": moejs.NativeFunc(func(_ *moejs.Realm, _ moejs.Value, args []moejs.Value) (moejs.Value, error) {
			key, data, encoding := moejs.Arg(args, 0), moejs.Arg(args, 1), moejs.Arg(args, 2)
			if !key.IsString() || !data.IsString() {
				return moejs.Undefined(), fmt.Errorf("crypto.hmacSha256: key and data must be strings")
			}

			keyBytes := []byte(key.String())
			if !encoding.IsUndefined() {
				if !encoding.IsString() || (encoding.String() != "utf8" && encoding.String() != "hex") {
					return moejs.Undefined(), fmt.Errorf("crypto.hmacSha256: key encoding must be utf8 or hex")
				}
				if encoding.String() == "hex" {
					var err error
					keyBytes, err = hex.DecodeString(key.String())
					if err != nil {
						return moejs.Undefined(), fmt.Errorf("crypto.hmacSha256: invalid hexadecimal key")
					}
				}
			}

			digest := hmac.New(sha256.New, keyBytes)
			_, _ = digest.Write([]byte(data.String()))
			return moejs.String(hex.EncodeToString(digest.Sum(nil))), nil
		}),
	}
}
