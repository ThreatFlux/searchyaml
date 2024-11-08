package storage

import (
	"bytes"
	"gopkg.in/yaml.v3"
	"sync"
)

// FastYAMLEncoder provides optimized YAML encoding with buffer pooling
type FastYAMLEncoder struct {
	pool *sync.Pool
}

// NewFastYAMLEncoder creates a new encoder with initialized buffer pool
func NewFastYAMLEncoder() *FastYAMLEncoder {
	return &FastYAMLEncoder{
		pool: &sync.Pool{
			New: func() interface{} {
				return &bytes.Buffer{}
			},
		},
	}
}

// Encode performs YAML encoding with buffer reuse
func (f *FastYAMLEncoder) Encode(v interface{}) ([]byte, error) {
	buf := f.pool.Get().(*bytes.Buffer)
	defer func() {
		buf.Reset()
		f.pool.Put(buf)
	}()

	encoder := yaml.NewEncoder(buf)
	encoder.SetIndent(2)

	if err := encoder.Encode(v); err != nil {
		return nil, err
	}

	encoder.Close()
	return buf.Bytes(), nil
}

// Decode performs YAML decoding with validation
func (f *FastYAMLEncoder) Decode(data []byte, v interface{}) error {
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	return decoder.Decode(v)
}
