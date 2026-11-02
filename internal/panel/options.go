package panel

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

// DecodeOptions decodes an options map into a typed struct via YAML
// re-marshal, so struct tags, defaults (pre-set on out), and types are
// honored. Unknown keys are an error to catch typos early.
func DecodeOptions(opts map[string]any, out any) error {
	if opts == nil {
		return nil
	}
	raw, err := yaml.Marshal(opts)
	if err != nil {
		return fmt.Errorf("encode options: %w", err)
	}
	dec := yaml.NewDecoder(bytesReader(raw))
	dec.KnownFields(true)
	if err := dec.Decode(out); err != nil {
		return fmt.Errorf("invalid options: %w", err)
	}
	return nil
}

// DecodeOptionsLoose is like DecodeOptions but ignores unknown keys, for
// options maps shared between a panel and its provider.
func DecodeOptionsLoose(opts map[string]any, out any) error {
	if opts == nil {
		return nil
	}
	raw, err := yaml.Marshal(opts)
	if err != nil {
		return fmt.Errorf("encode options: %w", err)
	}
	if err := yaml.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("invalid options: %w", err)
	}
	return nil
}
