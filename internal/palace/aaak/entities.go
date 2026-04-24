package aaak

import (
	cryptorand "crypto/rand"
	"encoding/binary"
	"fmt"
	"strings"
)

// GenerateCode creates a 3-char uppercase code for an entity name.
// If the natural code (first 3 chars) is taken, tries name[:2]+digit,
// then random 3-char codes.
func GenerateCode(name string, existing map[string]bool) string {
	if name == "" {
		return "UNK"
	}

	// Normalize: uppercase, letters only
	clean := strings.Map(func(r rune) rune {
		if r >= 'A' && r <= 'Z' {
			return r
		}
		if r >= 'a' && r <= 'z' {
			return r - ('a' - 'A')
		}
		return -1
	}, name)

	if len(clean) < 2 {
		clean = clean + "XX"
	}

	// Attempt 1: first 3 chars
	if len(clean) >= 3 {
		code := clean[:3]
		if !existing[code] {
			return code
		}
	}

	// Attempt 2: first 2 chars + digit (2-9)
	prefix := clean[:2]
	for d := 2; d <= 9; d++ {
		code := fmt.Sprintf("%s%d", prefix, d)
		if !existing[code] {
			return code
		}
	}

	// Attempt 3: random 3-char uppercase codes (crypto/rand for uniqueness)
	for i := 0; i < 100; i++ {
		var buf [2]byte
		_, _ = cryptorand.Read(buf[:])
		n := binary.LittleEndian.Uint16(buf[:])
		code := string([]byte{
			byte('A' + n%26),
			byte('A' + (n/26)%26),
			byte('A' + (n/676)%26),
		})
		if !existing[code] {
			return code
		}
	}

	// Fallback (should never reach with 17576 possible codes)
	return clean[:3]
}

