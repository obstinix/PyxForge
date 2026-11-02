package main

import (
	"crypto/md5"
	"encoding/hex"
)

// md5sum is a file's MD5, which a Debian package lists in md5sums for dpkg --verify. It checks
// that installed files are unchanged; it is no security measure.
func md5sum(b []byte) string {
	s := md5.Sum(b)
	return hex.EncodeToString(s[:])
}
