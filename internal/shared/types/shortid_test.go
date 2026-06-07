package types

import (
	"strings"
	"testing"
	"unicode"
)

func TestGenerateShortId(t *testing.T) {
	shortId := GenerateShortId()

	// Check length
	if len(shortId) != ShortIdLength {
		t.Errorf("expected length %d, got %d", ShortIdLength, len(shortId))
	}

	// Check charset
	for _, c := range shortId {
		if !strings.ContainsRune(ShortIdCharset, c) {
			t.Errorf("invalid character %c in ShortId", c)
		}
	}

	// Check no ambiguous characters
	if strings.ContainsAny(shortId, "IO") {
		t.Error("ShortId should not contain ambiguous characters I or O")
	}
}

func TestGenerateShortId_Uniqueness(t *testing.T) {
	generated := make(map[string]bool)
	count := 1000

	for i := 0; i < count; i++ {
		shortId := GenerateShortId()
		if generated[shortId] {
			t.Errorf("collision detected at iteration %d: %s", i, shortId)
		}
		generated[shortId] = true
	}
}

func TestValidateShortId_Valid(t *testing.T) {
	testCases := []string{
		"7K2T9",
		"3MWB7",
		"A1B2C",
		"12345",
		"ABCDE",
	}

	for _, tc := range testCases {
		if !ValidateShortId(tc) {
			t.Errorf("expected %s to be valid", tc)
		}
	}
}

func TestValidateShortId_Invalid(t *testing.T) {
	testCases := []string{
		"",            // empty
		"1234",        // too short
		"123456",      // too long
		"IO123",       // contains ambiguous I,O
		"abcde",       // lowercase not allowed
		"12-34",       // special character
		"12 34",       // space
	}

	for _, tc := range testCases {
		if ValidateShortId(tc) {
			t.Errorf("expected %s to be invalid", tc)
		}
	}
}

func TestShortIdCharset_NoAmbiguousChars(t *testing.T) {
	// Only I and O are considered truly ambiguous in this charset
	// 0 and 1 are commonly used and generally distinguishable
	ambiguous := []rune{'I', 'O'}

	for _, c := range ambiguous {
		if strings.ContainsRune(ShortIdCharset, c) {
			t.Errorf("ambiguous character %c should not be in charset", c)
		}
	}
}

func BenchmarkGenerateShortId(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = GenerateShortId()
	}
}

func BenchmarkValidateShortId(b *testing.B) {
	shortId := GenerateShortId()
	for i := 0; i < b.N; i++ {
		_ = ValidateShortId(shortId)
	}
	_ = unicode.ToLower('A') // use unicode to prevent compiler optimization
}
