package utils

import (
	"errors"
	"os"
)

func FileExists(pathWithFilename string) bool {
	_, err := os.Open(pathWithFilename)
	return !errors.Is(err, os.ErrNotExist)
}
