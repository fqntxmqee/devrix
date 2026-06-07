package types

import (
	"crypto/rand"
	"math/big"
)

// ShortIdCharset 是 ShortId 使用的字符集
// 去掉了易混淆的字符 I, O
const ShortIdCharset = "0123456789ABCDEFGHJKLMNPQRSTUVWXYZ"

const ShortIdLength = 5

// GenerateShortId 生成一个 5 位的短 ID
func GenerateShortId() string {
	b := make([]byte, ShortIdLength)
	charsetLen := big.NewInt(int64(len(ShortIdCharset)))
	for i := range b {
		n, err := rand.Int(rand.Reader, charsetLen)
		if err != nil {
			panic("shortid: crypto/rand failed: " + err.Error())
		}
		b[i] = ShortIdCharset[n.Int64()]
	}
	return string(b)
}

// ValidateShortId 验证 ShortId 格式是否正确
func ValidateShortId(shortId string) bool {
	if len(shortId) != ShortIdLength {
		return false
	}

	for _, c := range shortId {
		if !isValidShortIdChar(c) {
			return false
		}
	}

	return true
}

// isValidShortIdChar 检查字符是否在 ShortIdCharset 中
func isValidShortIdChar(c rune) bool {
	for _, charsetChar := range ShortIdCharset {
		if c == charsetChar {
			return true
		}
	}
	return false
}
