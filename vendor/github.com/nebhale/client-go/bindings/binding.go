/*
 * Copyright 2021 the original author or authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package bindings

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"

	"github.com/nebhale/client-go/internal"
)

// Provider is the key for the provider of a binding.
const Provider = "provider"

// Type is the key for the type of a binding.
const Type = "type"

// Binding is representation of a binding as defined by the Kubernetes Service Binding Specification:
// https://github.com/k8s-service-bindings/spec#workload-projection.
type Binding interface {

	// GetAsBytes returns the contents of a binding entry in its raw []byte form.
	GetAsBytes(key string) ([]byte, bool)

	// GetName returns the name of the binding.
	GetName() string
}

// Get returns contents of a binding entry as a UTF-8 decoded string.  Any whitespace is trimmed.
func Get(binding Binding, key string) (string, bool) {
	v, ok := binding.GetAsBytes(key)
	if !ok {
		return "", false
	}

	return strings.TrimSpace(string(v)), true
}

// GetProvider returns the value of the Provider key.
func GetProvider(binding Binding) (string, bool) {
	return Get(binding, Provider)
}

// GetType returns the value of the Type key.
func GetType(binding Binding) (string, error) {
	t, ok := Get(binding, Type)
	if !ok {
		return "", fmt.Errorf("binding does not contain a type")
	}

	return t, nil
}

// CacheBinding is an implementation of the Binding interface that caches values once they've been retrieved.
type CacheBinding struct {

	// Delegate is the Binding used to retrieve original values
	Delegate Binding

	cache map[string][]byte
	mutex sync.Mutex
}

func (c *CacheBinding) GetAsBytes(key string) ([]byte, bool) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if c.cache == nil {
		c.cache = make(map[string][]byte)
	}

	v, ok := c.cache[key]
	if ok {
		return v, ok
	}

	v, ok = c.Delegate.GetAsBytes(key)
	if ok {
		c.cache[key] = v
	}

	return v, ok
}

func (c *CacheBinding) GetName() string {
	return c.Delegate.GetName()
}

// ConfigTreeBinding is an implementation of the Binding interface that reads files from a volume mounted Kubernetes
// secret: https://kubernetes.io/docs/concepts/configuration/secret/#using-secrets.
type ConfigTreeBinding struct {

	// Root is the filesystem root of the binding.
	Root string
}

func (c ConfigTreeBinding) GetAsBytes(key string) ([]byte, bool) {
	if !internal.IsValidSecretKey(key) {
		return nil, false
	}

	p := filepath.Join(c.Root, key)

	if fi, err := os.Stat(p); err != nil || !fi.Mode().IsRegular() {
		return nil, false
	}

	b, err := os.ReadFile(p)
	if err != nil {
		return nil, false
	}

	return b, true
}

func (c ConfigTreeBinding) GetName() string {
	return path.Base(c.Root)
}

// MapBinding is an implementation of the Binding interface that returns values from a map.
type MapBinding struct {

	// Name is the name of the binding.
	Name string

	// Content is the content of the binding.
	Content map[string][]byte
}

func (m MapBinding) GetAsBytes(key string) ([]byte, bool) {
	if !internal.IsValidSecretKey(key) {
		return nil, false
	}

	v, ok := m.Content[key]
	return v, ok
}

func (m MapBinding) GetName() string {
	return m.Name
}
