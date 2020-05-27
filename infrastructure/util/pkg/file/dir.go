package file

import "os"

func IsDir(path string, create bool) (bool, error) {
	_, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			if create {
				if err := os.MkdirAll(path, 0777); err != nil {
					return false, err
				}
			}
			return false, nil
		}
		return false, nil
	}

	return true, err
}
