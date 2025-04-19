package fileutils

import "os"

// Returns nil if dirPath is a directory and is writable.
func VerifyWritable(dirPath string) error {
	fil, err := os.CreateTemp(dirPath, "")
	if err != nil {
		return err
	}
	err = fil.Close()
	if err != nil {
		return err
	}
	err = os.Remove(fil.Name())
	if err != nil {
		return err
	}
	return nil
}
