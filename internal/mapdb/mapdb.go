package mapdb

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/0xef53/kvmrun/internal/flock"
)

var (
	MIN_VALUE = 200
	MAX_VALUE = 65000

	ErrNoAvailableValues = errors.New("no available values")
)

type DB struct {
	file     string
	lockfile string
}

func New(fname string) *DB {
	return &DB{
		file:     fname,
		lockfile: fname + ".lock",
	}
}

func (db *DB) Get(key string) (int, error) {
	if err := os.MkdirAll(filepath.Dir(db.file), 0755); err != nil {
		return -1, err
	}

	lock, err := flock.NewLocker(db.lockfile)
	if err != nil {
		return -1, err
	}
	defer lock.Release()

	if err := lock.Acquire(time.Second * 10); err != nil {
		return -1, err
	}

	var m map[string]int

	if b, err := ioutil.ReadFile(db.file); err == nil {
		if err := json.Unmarshal(b, &m); err != nil {
			return -1, err
		}
	} else {
		if os.IsNotExist(err) {
			m = make(map[string]int)
		} else {
			return -1, err
		}
	}

	if v, ok := m[key]; ok {
		return v, nil
	}

	values := make([]int, 0, len(m))

	for _, v := range m {
		values = append(values, v)
	}

	if x, err := getVacantValue(values); err == nil {
		m[key] = x
	} else {
		return -1, err
	}

	if err := db.save(m); err != nil {
		return -1, err
	}

	return m[key], nil
}

func (db *DB) Delete(key string) (int, error) {
	lock, err := flock.NewLocker(db.lockfile)
	if err != nil {
		return -1, err
	}
	defer lock.Release()

	if err := lock.Acquire(time.Second * 10); err != nil {
		return -1, err
	}

	var m map[string]int

	if b, err := ioutil.ReadFile(db.file); err == nil {
		if err := json.Unmarshal(b, &m); err != nil {
			return -1, err
		}
	} else {
		if os.IsNotExist(err) {
			return -1, nil
		} else {
			return -1, err
		}
	}

	if v, ok := m[key]; ok {
		delete(m, key)

		if err := db.save(m); err != nil {
			return -1, err
		}

		return v, nil
	}

	return -1, nil
}

func (db *DB) save(m map[string]int) error {
	b, err := json.MarshalIndent(m, "", "    ")
	if err != nil {
		return err
	}

	return ioutil.WriteFile(db.file, b, 0644)
}

func getVacantValue(values []int) (int, error) {
	if len(values) == 0 {
		return MIN_VALUE, nil
	}

	tmp := make([]int, 0, len(values))

	contains := func(x int) bool {
		for _, v := range tmp {
			if v == x {
				return true
			}
		}
		return false
	}

	// Exclude out-of-bounds values
	for _, v := range values {
		if v >= MIN_VALUE && v <= MAX_VALUE && !contains(v) {
			tmp = append(tmp, v)
		}
	}

	switch {
	case len(tmp) == 1:
		if tmp[0] == MIN_VALUE {
			return tmp[0] + 1, nil
		} else {
			return MIN_VALUE, nil
		}
	case len(tmp) > 1:
		sort.Ints(tmp)

		if tmp[0] == MIN_VALUE {
			for idx := 0; idx < len(tmp)-1; idx++ {
				if tmp[idx]+1 != tmp[idx+1] {
					return tmp[idx] + 1, nil
				}
			}

			if tmp[len(tmp)-1] == MAX_VALUE {
				return -1, ErrNoAvailableValues
			}

			return tmp[len(tmp)-1] + 1, nil
		}
	}

	return MIN_VALUE, nil
}
