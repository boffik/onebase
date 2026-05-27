package gengen

import "gopkg.in/yaml.v3"

func init() {
	yamlUnmarshalImpl = func(data []byte, v interface{}) error {
		return yaml.Unmarshal(data, v)
	}
}
