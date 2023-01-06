package configuration

// Management controls software in the system.
//
// Example payload:
// {
//  "items": [
//    {
//      "package": "pkg1",
//      "service_name": "serviceName",
//      "config_files": [
//        {
//          "config_template": "configFileTemplate",
//          "config_location": "configFileLocation"
//        }
//      ],
//      "parameters": [
//        {
//          "key": "configKey",
//          "value": "configValue"
//        }
//      ]
//    }
//  ]
// }
type Management struct {
	Metadata

	Items []Software `json:"items"`
}

// Software defines software to be maintained in the system.
type Software struct {
	// Package defines a package name to install.
	Package string `json:"package"`

	// ServiceName defines an optional service name (if empty, Package is used).
	ServiceName string `json:"service_name"`

	// ConfigFiles to be created for the software.
	ConfigFiles []ConfigFile `json:"config_files"`

	// Parameters for the ConfigFiles templating.
	Parameters []ConfigFileParameter `json:"parameters"`
}

// ConfigFile definition.
type ConfigFile struct {
	// ConfigTemplate defines a source template file from file manager.
	ConfigTemplate string `json:"config_template"`

	// ConfigLocation defines an absolute path in the system where file will be created.
	ConfigLocation string `json:"config_location"`
}

// ConfigFileParameter defines parameter to be used in ConfigFile.
type ConfigFileParameter struct {
	// Key defines parameters name.
	Key string `json:"key"`

	// Value defines parameters value.
	Value string `json:"value"`
}
