package aperture

import "github.com/zoobzio/capitan"

// Variant name constants for Spec output.
const (
	variantNameString   = "string"
	variantNameInt      = "int"
	variantNameInt32    = "int32"
	variantNameInt64    = "int64"
	variantNameUint     = "uint"
	variantNameUint32   = "uint32"
	variantNameUint64   = "uint64"
	variantNameFloat32  = "float32"
	variantNameFloat64  = "float64"
	variantNameBool     = "bool"
	variantNameTime     = "time"
	variantNameDuration = "duration"
	variantNameBytes    = "bytes"
	variantNameError    = "error"
)

// Spec provides introspection into registered components.
// Useful for tooling, validation, and documentation generation.
type Spec struct {
	Signals      []SignalSpec      `json:"signals"`
	Keys         []KeySpec         `json:"keys"`
	ContextKeys  []string          `json:"context_keys"`
	Transformers []TransformerSpec `json:"transformers"`
}

// SignalSpec describes a registered signal.
type SignalSpec struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// KeySpec describes a registered field key.
type KeySpec struct {
	Name    string `json:"name"`
	Variant string `json:"variant"`
}

// TransformerSpec describes a registered transformer.
type TransformerSpec struct {
	Variant string `json:"variant"`
}

// Spec returns introspection data for all registered components.
func (r *Registry) Spec() Spec {
	spec := Spec{
		Signals:      make([]SignalSpec, 0, len(r.signals)),
		Keys:         make([]KeySpec, 0, len(r.keys)),
		ContextKeys:  make([]string, 0, len(r.contextKeys)),
		Transformers: make([]TransformerSpec, 0, len(r.transformers)),
	}

	for _, s := range r.signals {
		spec.Signals = append(spec.Signals, SignalSpec{
			Name:        s.Name(),
			Description: s.Description(),
		})
	}

	for _, k := range r.keys {
		spec.Keys = append(spec.Keys, KeySpec{
			Name:    k.Name(),
			Variant: variantName(k.Variant()),
		})
	}

	for name := range r.contextKeys {
		spec.ContextKeys = append(spec.ContextKeys, name)
	}

	for v := range r.transformers {
		spec.Transformers = append(spec.Transformers, TransformerSpec{
			Variant: variantName(v),
		})
	}

	return spec
}

// variantName returns a string representation of a capitan Variant.
func variantName(v capitan.Variant) string {
	// Built-in variants have known names
	switch v {
	case capitan.VariantString:
		return variantNameString
	case capitan.VariantInt:
		return variantNameInt
	case capitan.VariantInt32:
		return variantNameInt32
	case capitan.VariantInt64:
		return variantNameInt64
	case capitan.VariantUint:
		return variantNameUint
	case capitan.VariantUint32:
		return variantNameUint32
	case capitan.VariantUint64:
		return variantNameUint64
	case capitan.VariantFloat32:
		return variantNameFloat32
	case capitan.VariantFloat64:
		return variantNameFloat64
	case capitan.VariantBool:
		return variantNameBool
	case capitan.VariantTime:
		return variantNameTime
	case capitan.VariantDuration:
		return variantNameDuration
	case capitan.VariantBytes:
		return variantNameBytes
	case capitan.VariantError:
		return variantNameError
	default:
		return string(v)
	}
}
