package utils

import (
	"errors"
	"fmt"
	"log"
	"log/slog"
	"os"
	"path"
	"path/filepath"
)

func FileExists(pathWithFilename string) bool {
	// Ensure we are only reading files from our executable and not where the terminal is executing from.
	execPath, err := os.Executable()
	if err != nil {
		log.Fatal(err)
	}
	execDirPath := path.Dir(execPath)
	fullFilePath := filepath.Join(execDirPath, pathWithFilename)
	slog.Debug("FILE EXIST?: %s", fullFilePath)

	_, err = os.Open(fullFilePath)
	return !errors.Is(err, os.ErrNotExist)
}

// Called when we want to provide a file path for an Emissary resource or config file.
// This is common when we want to use a library which requires a path to our file, but we aren't
// reading from the file e.g tls.LoadX509KeyPair()
func CreateEmissaryFileReadPath(relativePathWithFilename string) string {
	// Ensure we are only reading files from our executable and not where the terminal is executing from.
	execPath, err := os.Executable()
	if err != nil {
		log.Fatal(err)
	}
	execDirPath := path.Dir(execPath)
	fullFilePath := filepath.Join(execDirPath, relativePathWithFilename)

	return fullFilePath
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

func ReadFile(pathWithFilename string) *[]byte {
	// Ensure we are only reading files from our executable and not where the terminal is executing from.
	execPath, err := os.Executable()
	if err != nil {
		log.Fatal(err)
	}
	execDirPath := path.Dir(execPath)
	fullFilePath := filepath.Join(execDirPath, pathWithFilename)
	slog.Debug("File Operation", slog.String("Read File", fullFilePath))

	file, err := os.ReadFile(fullFilePath)
	if !errors.Is(err, os.ErrNotExist) {
		return &file
	}
	return nil
}
