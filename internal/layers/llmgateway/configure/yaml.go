package configure

import "gopkg.in/yaml.v3"

// unmarshalYAML is a thin wrapper so tests and embedded decoders share one
// yaml.v3 instance.
func unmarshalYAML(data []byte, v interface{}) error {
	return yaml.Unmarshal(data, v)
}