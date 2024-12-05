package fileutils

import "os"

// Returns nil if dirPath is a directory and is writable.
func VerifyWritable(dirPath string) error {
	fil, err := os.CreateTemp(dirPath, "")
	if err != nil {
		return err
	}
	fil.Close()
	err = os.Remove(fil.Name())
	if err != nil {
		return err
	}
	return nil
}
