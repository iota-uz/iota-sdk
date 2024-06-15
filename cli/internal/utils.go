package internal

import "os"

func FileExists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

func DirExists(dir string) bool {
	info, err := os.Stat(dir)
	if os.IsNotExist(err) {
		return false
	}
	return info.IsDir()
}

func MkDirIfNone(dir string) error {
	if DirExists(dir) {
		return nil
	}
	return os.MkdirAll(dir, 0755)
}
