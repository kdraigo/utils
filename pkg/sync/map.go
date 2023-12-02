package sync

import (
	"encoding/json"
	"sync"
)

// Map implementing thread safe map[K]V.
type Map[KeyType comparable, ValueType any] struct {
	data      map[KeyType]ValueType
	dataGuard *sync.RWMutex
}

// NewMap creating Map instance.
func NewMap[KeyType comparable, ValueType any](size int) *Map[KeyType, ValueType] {
	return &Map[KeyType, ValueType]{
		data:      make(map[KeyType]ValueType, size),
		dataGuard: &sync.RWMutex{},
	}
}

// WrapMap setting existing map under guard. Unsafe.
func WrapMap[KeyType comparable, ValueType any](data map[KeyType]ValueType) *Map[KeyType, ValueType] {
	if len(data) == 0 {
		data = make(map[KeyType]ValueType)
	}
	return &Map[KeyType, ValueType]{
		data:      data,
		dataGuard: &sync.RWMutex{},
	}
}

// AddElement adding new element under key or overriding it.
func (sm *Map[KeyType, ValueType]) AddElement(key KeyType, value ValueType) {
	sm.dataGuard.Lock()
	defer sm.dataGuard.Unlock()

	sm.data[key] = value
}

// AddElements adding new elements in map under key or overriding that elements.
func (sm *Map[KeyType, ValueType]) AddElements(elements map[KeyType]ValueType) {
	sm.dataGuard.Lock()
	defer sm.dataGuard.Unlock()

	for key, value := range elements {
		sm.data[key] = value
	}
}

// TryGetElement trying to get element if exists.
func (sm *Map[KeyType, ValueType]) TryGetElement(key KeyType) (ValueType, bool) {
	sm.dataGuard.RLock()
	defer sm.dataGuard.RUnlock()

	data, ok := sm.data[key]
	return data, ok
}

// TryGetElements trying to get elements if exist.
func (sm *Map[KeyType, ValueType]) TryGetElements(keys []KeyType) map[KeyType]ValueType {
	sm.dataGuard.RLock()
	defer sm.dataGuard.RUnlock()

	desired := make(map[KeyType]ValueType)
	for _, key := range keys {
		if value, ok := sm.data[key]; ok {
			desired[key] = value
		}
	}
	return desired
}

// GetKeys getting all keys from map.
func (sm *Map[KeyType, ValueType]) GetKeys() []KeyType {
	sm.dataGuard.RLock()
	defer sm.dataGuard.RUnlock()

	var keys []KeyType
	for k := range sm.data {
		keys = append(keys, k)
	}

	return keys
}

// Remove deleting element under key.
func (sm *Map[KeyType, ValueType]) Remove(key KeyType) ValueType {
	sm.dataGuard.RLock()
	defer sm.dataGuard.RUnlock()

	value := sm.data[key]

	delete(sm.data, key)
	return value
}

// Cleanup deleting all elements from map.
func (sm *Map[KeyType, ValueType]) Cleanup() int {
	sm.dataGuard.RLock()
	defer sm.dataGuard.RUnlock()

	count := 0
	for key := range sm.data {
		delete(sm.data, key)
		count++
	}
	return count
}

func (sm *Map[KeyType, ValueType]) MarshalJSON() ([]byte, error) {
	sm.dataGuard.RLock()
	defer sm.dataGuard.RUnlock()

	return json.Marshal(sm.data)
}
