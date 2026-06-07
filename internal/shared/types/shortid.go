package types

import (
	"math/rand"
	"time"
)

// ShortIdCharset 是 ShortId 使用的字符集
// 去掉了易混淆的字符 I, O
const ShortIdCharset = "0123456789ABCDEFGHJKLMNPQRSTUVWXYZ"

const ShortIdLength = 5

// GenerateShortId 生成一个 5 位的短 ID
// 使用时间戳 + 随机数确保唯一性
func GenerateShortId() string {
	b := make([]byte, ShortIdLength)
	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	// 使用加密强度较低的随机数生成器，对于 ShortId 场景足够
	for i := range b {
		b[i] = ShortIdCharset[r.Intn(len(ShortIdCharset))]
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
