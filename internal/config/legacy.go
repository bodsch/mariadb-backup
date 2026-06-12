package config

import "gopkg.in/yaml.v3"

// knownKeys describes the accepted configuration schema. A nil value means the
// key is a leaf (its sub-tree is not inspected, e.g. free-form lists/scalars).
var knownKeys = map[string]any{
	"connection": map[string]any{
		"username": nil,
		"password": nil,
		"host":     nil,
		"port":     nil,
		"socket":   nil,
	},
	"storage": map[string]any{
		"destination": nil,
		"compression": nil,
		"rotation": map[string]any{
			"daily":  nil,
			"weekly": nil,
		},
	},
	"notification": map[string]any{
		"enabled": nil,
		"smtp": map[string]any{
			"server_name": nil,
			"port":        nil,
			"tls":         nil,
			"auth": map[string]any{
				"username": nil,
				"password": nil,
			},
		},
		"sender":    nil,
		"recipient": nil,
	},
	"excludes": map[string]any{
		"databases": nil,
		"tables":    nil,
	},
}

// detectLegacyKeys returns the dotted paths of keys present in the YAML document
// that are not part of the known schema (typo'd or removed keys, the dropped
// `includes` block, etc.).
func detectLegacyKeys(data []byte) []string {
	var raw map[string]any
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil
	}
	var out []string
	walkUnknown("", raw, knownKeys, &out)
	return out
}

func walkUnknown(prefix string, actual map[string]any, known map[string]any, out *[]string) {
	for k, v := range actual {
		path := k
		if prefix != "" {
			path = prefix + "." + k
		}
		sub, ok := known[k]
		if !ok {
			*out = append(*out, path)
			continue
		}
		// Known key. Recurse only when both sides are maps.
		subKnown, knownIsMap := sub.(map[string]any)
		subActual, actualIsMap := v.(map[string]any)
		if knownIsMap && actualIsMap {
			walkUnknown(path, subActual, subKnown, out)
		}
	}
}
