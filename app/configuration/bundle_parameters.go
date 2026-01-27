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
	"fmt"
	"os"
	"path"
	"strings"

	"go.qbee.io/agent/app/inventory"
	"go.qbee.io/agent/app/software"
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

// URLSigner is an interface for signing URLs.
type URLSigner interface {
	SignURL(url string) (string, error)
}

// ParameterStore defines a key->value map of parameters, as well as URL signer.
type ParameterStore struct {
	values    map[string]string
	urlSigner URLSigner
}

const (
	parameterKeyOpen       = "$("
	parameterKeyClose      = ')'
	parameterKeyFilePrefix = "file://"
)

var systemParameters = map[string]func(ctx context.Context) (string, error){
	"sys.host": func(ctx context.Context) (string, error) {
		return os.Hostname()
	},
	"sys.pkg_arch": func(ctx context.Context) (string, error) {
		if software.DefaultPackageManager == nil {
			return "", fmt.Errorf("package manager is not supported")
		}
		return software.DefaultPackageManager.PackageArchitecture(ctx)
	},
	"sys.pkg_type": func(ctx context.Context) (string, error) {
		if software.DefaultPackageManager == nil {
			return "", fmt.Errorf("package manager is not supported")
		}
		return string(software.DefaultPackageManager.Type()), nil
	},
	"sys.os": func(ctx context.Context) (string, error) {
		systemInventory, err := inventory.CollectSystemInventory(false)
		if err != nil {
			return "", err
		}
		return systemInventory.System.OS, nil
	},
	"sys.arch": func(ctx context.Context) (string, error) {
		systemInventory, err := inventory.CollectSystemInventory(false)
		if err != nil {
			return "", err
		}
		return systemInventory.System.Architecture, nil
	},
	"sys.os_type": func(ctx context.Context) (string, error) {
		systemInventory, err := inventory.CollectSystemInventory(false)
		if err != nil {
			return "", err
		}
		return systemInventory.System.OSType, nil
	},
	"sys.flavor": func(ctx context.Context) (string, error) {
		systemInventory, err := inventory.CollectSystemInventory(false)
		if err != nil {
			return "", err
		}
		return systemInventory.System.Flavor, nil
	},
	"sys.agent_version": func(ctx context.Context) (string, error) {
		systemInventory, err := inventory.CollectSystemInventory(false)
		if err != nil {
			return "", err
		}
		return systemInventory.System.AgentVersion, nil
	},
	"sys.long_arch": func(ctx context.Context) (string, error) {
		systemInventory, err := inventory.CollectSystemInventory(false)
		if err != nil {
			return "", err
		}
		return systemInventory.System.LongArchitecture, nil
	},
	"sys.boot_time": func(ctx context.Context) (string, error) {
		systemInventory, err := inventory.CollectSystemInventory(false)
		if err != nil {
			return "", err
		}
		return systemInventory.System.BootTime, nil
	},
}

// resolveParameter given context with parameter store attached, returns resolved parameter value.
func resolveParameters(ctx context.Context, value string) string {
	parameterStore, ok := ctx.Value(ctxParameterStore).(*ParameterStore)
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

		if strings.HasPrefix(key, parameterKeyFilePrefix) {
			filePath := strings.TrimPrefix(strings.TrimPrefix(key, parameterKeyFilePrefix), "/")
			filePath = path.Join(fileManagerPublicAPIPath, filePath)
			signedURL, err := parameterStore.urlSigner.SignURL(filePath)
			if err != nil {
				ReportError(ctx, err, "cannot sign URL for %s", filePath)
				result.WriteString(value[start : i+1])
				continue
			} else {
				result.WriteString(signedURL)
				continue
			}
		}

		// Lookup in the parameter store and use if found.
		if val, exists := parameterStore.values[key]; exists {
			result.WriteString(val)
			continue
		}

		// Lookup in the system parameters and use if found.
		if valFn, exists := systemParameters[key]; exists {
			if val, err := valFn(ctx); err != nil {
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
func (parameters *ParametersBundle) Context(ctx context.Context, urlSigner URLSigner) context.Context {

	parametersStore := &ParameterStore{
		urlSigner: urlSigner,
		values:    make(map[string]string),
	}

	for _, parameter := range parameters.Parameters {
		parametersStore.values[parameter.Key] = parameter.Value
	}

	for _, secret := range parameters.Secrets {
		parametersStore.values[secret.Key] = secret.Value
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
