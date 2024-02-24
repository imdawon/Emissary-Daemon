package utils

import (
	"errors"
	"fmt"
	"log"
	"os"
)

func FileExists(pathWithFilename string) bool {
	_, err := os.Open(pathWithFilename)
	return !errors.Is(err, os.ErrNotExist)
}

func PrintFinalError(message string, err error) {
	if message == "" {
		message = "A fatal error occurred"
	}
	if err == nil {
		log.Printf(message)
	} else {
		log.Printf("%s: %s", message, err)
	}
	fmt.Println("Press Enter key to exit...")
	var noop string
	fmt.Scanln(&noop)
	os.Exit(1)
}
