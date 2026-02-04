package dotenv

import (
	"bytes"
	"io"
	"os"
)

// Read reads all specified .env files, parses their contents and returns values as a map.
//
// If no file names given, the function will try to read the .env file located in the current path.
func Read(fnames ...string) (map[string]string, error) {
	if len(fnames) == 0 {
		fnames = []string{".env"}
	}

	envs := make(map[string]string)

	for _, fname := range fnames {
		m, err := readFile(fname)
		if err != nil {
			return nil, err
		}

		for key, value := range m {
			envs[key] = value
		}
	}

	return envs, nil
}

// Read reads a specified .env file and returns values as a map.
func readFile(fname string) (map[string]string, error) {
	file, err := os.Open(fname)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	return Parse(file)
}

// Parse reads a data of specified .env file from io.Reader and returns a map of keys and values.
func Parse(r io.Reader) (map[string]string, error) {
	var buf bytes.Buffer

	_, err := io.Copy(&buf, r)
	if err != nil {
		return nil, err
	}

	m := make(map[string]string)

	if err := parseBytes(buf.Bytes(), m); err != nil {
		return nil, err
	}

	return m, nil
}
