package attributes

import (
	"fmt"
	"sort"
	"strings"
)

// DeviceAttributes matches the flat-object JSON format used by the API:
//
//	{"device_name":"...","longitude":"...","latitude":"...","custom":{"key":"value",...}}
type DeviceAttributes struct {
	DeviceName string            `json:"device_name,omitempty"`
	Longitude  string            `json:"longitude,omitempty"`
	Latitude   string            `json:"latitude,omitempty"`
	Custom     map[string]string `json:"custom,omitempty"`
}

// setters and getters for predefined attributes. Custom attributes are handled separately.
// This is to avoid reflection and keep the code simple and efficient, as the set of predefined attributes is fixed and small.
// but at the same time allow for easy addition of new predefined attributes in the future without changing the core logic.
var attributeSetters = map[string]func(attrs *DeviceAttributes, value string){
	"device_name": func(attrs *DeviceAttributes, value string) { attrs.DeviceName = value },
	"longitude":   func(attrs *DeviceAttributes, value string) { attrs.Longitude = value },
	"latitude":    func(attrs *DeviceAttributes, value string) { attrs.Latitude = value },
}

var attributeGetters = map[string]func(attrs *DeviceAttributes) (string, bool){
	"device_name": func(attrs *DeviceAttributes) (string, bool) { return attrs.DeviceName, attrs.DeviceName != "" },
	"longitude":   func(attrs *DeviceAttributes) (string, bool) { return attrs.Longitude, attrs.Longitude != "" },
	"latitude":    func(attrs *DeviceAttributes) (string, bool) { return attrs.Latitude, attrs.Latitude != "" },
}

// Filter returns a map containing only the specified keys with their values, using the same
// nested structure as the full DeviceAttributes JSON (custom.* keys are nested under "custom").
// Unknown or missing keys are silently omitted.
func (d DeviceAttributes) Filter(keys []string) map[string]any {
	result := make(map[string]any)
	custom := make(map[string]string)

	for _, key := range keys {
		value, ok := d.GetValue(key)
		if !ok || value == "" {
			continue
		}

		if strings.HasPrefix(key, "custom.") {
			suffix := key[len("custom."):]
			custom[suffix] = value
		} else {
			result[key] = value
		}
	}

	if len(custom) > 0 {
		result["custom"] = custom
	}

	return result
}

const attributeShellVarPrefix = "QBEE_ATTRIBUTE_"
const customAttributeShellVarPrefix = "QBEE_ATTRIBUTE_CUSTOM_"

// ToShellVarName converts an attribute key to its QBEE_ATTRIBUTE_ shell variable name.
// Custom attributes (custom.*) get the QBEE_ATTRIBUTE_CUSTOM_ prefix.
// All other allowed attributes get the QBEE_ATTRIBUTE_ prefix.
// The key is uppercased and dots/dashes are replaced with underscores.
func ToShellVarName(key string) string {
	prefix := attributeShellVarPrefix
	lowerCaseSuffix := key

	if strings.HasPrefix(key, "custom.") {
		lowerCaseSuffix = key[len("custom."):]
		prefix = customAttributeShellVarPrefix
	}

	suffix := strings.ToUpper(strings.NewReplacer(".", "_", "-", "_").Replace(lowerCaseSuffix))

	return prefix + suffix
}

// ShellLines returns QBEE_ATTRIBUTE_*="value" lines suitable for shell sourcing.
// Predefined attributes are always included; custom attributes are sorted for determinism.
func (d DeviceAttributes) ShellLines() []string {
	lines := make([]string, 0)

	for key, getter := range attributeGetters {
		if value, ok := getter(&d); ok && value != "" {
			line := fmt.Sprintf("%s=%q", ToShellVarName(key), value)
			lines = append(lines, line)
		}
	}

	if d.Custom == nil {
		return lines
	}

	// Sort custom keys for deterministic output.
	keys := make([]string, 0, len(d.Custom))
	for k := range d.Custom {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	for _, k := range keys {
		lines = append(lines, fmt.Sprintf("%s=%q", ToShellVarName("custom."+k), d.Custom[k]))
	}

	return lines
}

// GetValue returns the value for a given normalized attribute key (e.g. "device_name", "custom.mykey").
// Returns ("", false) if the key is not present.
func (d DeviceAttributes) GetValue(key string) (string, bool) {
	if getter, ok := attributeGetters[key]; ok {
		return getter(&d)
	}
	if !strings.HasPrefix(key, "custom.") || d.Custom == nil {
		return "", false
	}

	suffix := key[len("custom."):]
	if v, ok := d.Custom[suffix]; ok {
		return v, true
	}

	return "", false
}

func ParseKeyValueArgs(args []string) (*DeviceAttributes, error) {
	attrs := &DeviceAttributes{}

	for _, arg := range args {
		parts := strings.SplitN(arg, "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid key=value pair: %q", arg)
		}

		if setter, ok := attributeSetters[parts[0]]; ok {
			setter(attrs, parts[1])
			continue
		}

		if !strings.HasPrefix(parts[0], "custom.") {
			continue
		}
		if attrs.Custom == nil {
			attrs.Custom = make(map[string]string)
		}
		attrs.Custom[parts[0][len("custom."):]] = parts[1]

	}

	return attrs, nil
}
