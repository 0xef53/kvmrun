package pool

/*
	TODO: write tests
*/

import (
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"sync"
)

type Pool struct {
	sync.Mutex

	items map[string]interface{}
	keys  []string
}

func (p *Pool) init() {
	if p.items == nil {
		p.items = make(map[string]interface{})

		p.keys = make([]string, 0, 1)
	}
}

func (p *Pool) Keys() []string {
	p.Lock()
	defer p.Unlock()

	return slices.Clone(p.keys)
}

func (p *Pool) Values(keys ...string) []interface{} {
	p.Lock()
	defer p.Unlock()

	return p.values(keys...)
}

func (p *Pool) values(keys ...string) []interface{} {
	if len(keys) == 0 {
		keys = p.keys
	}

	valueList := make([]interface{}, 0, len(keys))

	for _, key := range keys {
		valueList = append(valueList, p.items[key])
	}

	return valueList
}

func (p *Pool) Len() int {
	return len(p.keys)
}

func (p *Pool) GetAs(key string, target interface{}) error {
	p.Lock()
	defer p.Unlock()

	if target == nil {
		return fmt.Errorf("target must be a non-nil pointer")
	}

	if p.items == nil {
		return fmt.Errorf("%w: %s", ErrNotFound, key)
	}

	if _, ok := p.items[key]; !ok {
		return fmt.Errorf("%w: %s", ErrNotFound, key)
	}

	targetRV := reflect.ValueOf(target)

	if targetRV.Kind() != reflect.Ptr || targetRV.IsNil() {
		return fmt.Errorf("target must be a non-nil pointer to a variable")
	}

	targetElem := targetRV.Elem()

	if !targetElem.CanSet() {
		return fmt.Errorf("target is not settable")
	}

	value := p.items[key]

	if value == nil {
		return fmt.Errorf("value is nil (no type information): key = %s", key)
	}

	valueRV := reflect.ValueOf(value)

	// Is value from items can be assigned to the target ?
	if !valueRV.Type().AssignableTo(targetElem.Type()) {
		return fmt.Errorf("type mismatch: value type = %s, target type = %s", valueRV.Type(), targetElem.Type())
	}

	targetElem.Set(valueRV)

	return nil
}

func (p *Pool) Exists(key string) bool {
	p.Lock()
	defer p.Unlock()

	if p.items == nil {
		return false
	}

	_, ok := p.items[key]

	return ok
}

func (p *Pool) Append(key string, value interface{}, replace bool) error {
	p.Lock()
	defer p.Unlock()

	if len(key) == 0 {
		return fmt.Errorf("key cannot be empty")
	}

	p.init()

	if _, ok := p.items[key]; ok && !replace {
		return fmt.Errorf("%w: %s", ErrAlreadyExists, key)
	}

	p.items[key] = value

	p.keys = append(p.keys, key)

	return nil
}

func (p *Pool) Insert(key string, value interface{}, position int) error {
	p.Lock()
	defer p.Unlock()

	if len(key) == 0 {
		return fmt.Errorf("key cannot be empty")
	}

	if position < 0 {
		position = 0
	}

	p.init()

	if _, ok := p.items[key]; ok {
		return fmt.Errorf("%w: %s", ErrAlreadyExists, key)
	}

	p.items[key] = value

	if n := len(p.keys); position > n {
		position = n
	}

	p.keys = slices.Insert(p.keys, position, key)

	p.items[key] = value

	return nil
}

func (p *Pool) Remove(key string) error {
	p.Lock()
	defer p.Unlock()

	if p.items != nil {
		if _, ok := p.items[key]; ok {
			p.keys = slices.DeleteFunc(p.keys, func(s string) bool {
				return s == key
			})

			delete(p.items, key)

			return nil
		}
	}

	return fmt.Errorf("%w: %s", ErrNotFound, key)
}

func (p *Pool) RemoveN(idx int) error {
	p.Lock()
	defer p.Unlock()

	if idx < 0 {
		return fmt.Errorf("idx cannot be less than 0")
	}
	if idx > len(p.keys) {
		return fmt.Errorf("idx cannot be greater than pool length")
	}

	if p.items != nil {
		key := p.keys[idx]

		p.keys = slices.Delete(p.keys, idx, idx+1)

		delete(p.items, key)
	}

	return nil
}

func (p *Pool) MarshalJSON() ([]byte, error) {
	p.Lock()
	defer p.Unlock()

	return json.Marshal(p.values())
}
