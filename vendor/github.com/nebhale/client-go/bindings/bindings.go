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
	"os"
	"path/filepath"
	"strings"
)

// ServiceBindingRoot is the name of the environment variable read to determine the bindings file system root.
// Specified by the Kubernetes Service Binding Specification.
const ServiceBindingRoot = "SERVICE_BINDING_ROOT"

// Cached wraps each Binding in a CachedBinding.
func Cached(bindings []Binding) []Binding {
	var w []Binding

	for _, b := range bindings {
		w = append(w, &CacheBinding{Delegate: b})
	}

	return w
}

// From creates a collection Bindings from the specified path.  If the directory does not exist an, empty collection is
// returned.
func From(root string) []Binding {
	if fi, err := os.Stat(root); err != nil || !fi.IsDir() {
		return []Binding{}
	}

	children, err := os.ReadDir(root)
	if err != nil {
		return []Binding{}
	}

	var bindings []Binding
	for _, c := range children {
		if !c.IsDir() {
			continue
		}

		bindings = append(bindings, ConfigTreeBinding{Root: filepath.Join(root, c.Name())})
	}

	return bindings
}

// FromServiceBindingRoot creates Bindings using the $SERVICE_BINDING_ROOT environment variable to determine the file
// system toot.  If the $SERVICE_BINDING_ROOT environment variable is not set, an empty collection is returned.  If the
// directory does not exist, an empty collection is returned.
func FromServiceBindingRoot() []Binding {
	path, ok := os.LookupEnv(ServiceBindingRoot)
	if !ok {
		return []Binding{}
	}

	return From(path)
}

// Find returns a Binding with a given name.  Comparison is case-insensitive.
func Find(bindings []Binding, name string) (Binding, bool) {
	for _, b := range bindings {
		if strings.EqualFold(b.GetName(), name) {
			return b, true
		}
	}

	return nil, false
}

// FilterWithProvider returns zero or more Bindings with a given type and provider.  If type or provider are empty, the
// result is not filtered on that argument.  Comparisons are case-insensitive.
func FilterWithProvider(bindings []Binding, bindingType string, provider string) []Binding {
	var match []Binding

	for _, b := range bindings {
		if bindingType != "" {
			if t, err := GetType(b); err != nil || !strings.EqualFold(bindingType, t) {
				continue
			}
		}

		if provider != "" {
			if p, ok := GetProvider(b); !ok || !strings.EqualFold(provider, p) {
				continue
			}
		}

		match = append(match, b)
	}

	return match
}

// Filter returns zero or more Bindings with a given type.  Equivalent to FilterWithProvider with an empty provider.
func Filter(bindings []Binding, bindingType string) []Binding {
	return FilterWithProvider(bindings, bindingType, "")
}
