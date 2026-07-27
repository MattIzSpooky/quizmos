package game

import (
	"context"
	"crypto/rand"
	"fmt"
)

// alphabet excludes visually ambiguous characters (0/O, 1/I/L).
const codeAlphabet = "ABCDEFGHJKMNPQRSTUVWXYZ23456789"
const codeLength = 6

func randomCode() (string, error) {
	buf := make([]byte, codeLength)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	code := make([]byte, codeLength)
	for i, b := range buf {
		code[i] = codeAlphabet[int(b)%len(codeAlphabet)]
	}
	return string(code), nil
}

func (s *Service) generateUniqueGameCode(ctx context.Context) (string, error) {
	for attempt := 0; attempt < 10; attempt++ {
		code, err := randomCode()
		if err != nil {
			return "", err
		}
		exists, err := s.q.GameCodeExists(ctx, code)
		if err != nil {
			return "", err
		}
		if !exists {
			return code, nil
		}
	}
	return "", fmt.Errorf("could not generate a unique game code after 10 attempts")
}
