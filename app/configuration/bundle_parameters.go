// Copyright 2023 qbee.io
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
// SPDX-License-Identifier: Apache-2.0

package configuration

import (
	"context"
	"os"
	"strings"
)

// Parameter defines a parameters as key/value pair.
type Parameter struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

const ctxParameterStore = contextKey("configuration:parameter-store")

// ParametersBundle defines global system parameters.
//
// Example payload:
//
//		{
//		 "parameters": [
//		   {
//		     "key": "placeholder",
//		     "value": "value"
//		   }
//		 ],
//	  "secrets": [
//		   {
//		     "key": "placeholder",
//		     "value": "value"
//		   }
//		 ]
//		}
type ParametersBundle struct {
	Metadata

	Parameters []Parameter `json:"parameters"`
	Secrets    []Parameter `json:"secrets"`
}

// ParameterStore defines a key->value map of parameters.
type ParameterStore map[string]string

const (
	parameterKeyOpen  = "$("
	parameterKeyClose = ')'
)

var systemParameters = map[string]func() (string, error){
	"sys.host": os.Hostname,
}

// resolveParameter given context with parameter store attached, returns resolved parameter value.
func resolveParameters(ctx context.Context, value string) string {
	parameterStore, ok := ctx.Value(ctxParameterStore).(ParameterStore)
	if !ok {
		ReportError(ctx, "cannot resolve parameter", "parameter store is not set in context")
		return value
	}

	var result strings.Builder
	length := len(value)

	for i := 0; i < length; i++ {
		// Check if we have a '$(', if not, append to the result.
		if i+1 >= length || value[i:i+2] != parameterKeyOpen {
			result.WriteByte(value[i])
			continue
		}

		start := i

		i += 2 // Skip '$('
		startParam := i

		// Find the closing ')'
		for i < length && value[i] != parameterKeyClose {
			i++
		}

		// Check if we found a closing ')', if not, just append the rest of the string and break
		if i >= length {
			result.WriteString(value[start:])
			break
		}

		// Extract parameter key
		key := value[startParam:i]

		// Lookup in the parameter store and use if found.
		if val, exists := parameterStore[key]; exists {
			result.WriteString(val)
			continue
		}

		// Lookup in the system parameters and use if found.
		if valFn, exists := systemParameters[key]; exists {
			if val, err := valFn(); err != nil {
				ReportError(ctx, err, "cannot resolve parameter %s", key)
				result.WriteString(value[start : i+1])
			} else {
				result.WriteString(val)
			}

			continue
		}

		// If not found in either parameter store nor system parameters, leave it as is.
		result.WriteString(value[start : i+1])
	}

	return result.String()
}

// Context returns a new context based on parent context with parameter store attached.
func (parameters *ParametersBundle) Context(ctx context.Context) context.Context {
	parametersStore := make(ParameterStore)

	for _, parameter := range parameters.Parameters {
		parametersStore[parameter.Key] = parameter.Value
	}

	for _, secret := range parameters.Secrets {
		parametersStore[secret.Key] = secret.Value
	}

	return context.WithValue(ctx, ctxParameterStore, parametersStore)
}

// SecretsList returns a list of all secrets.
func (parameters *ParametersBundle) SecretsList() []string {
	var secrets []string

	for _, secret := range parameters.Secrets {
		secrets = append(secrets, secret.Value)
	}

	return secrets
}
