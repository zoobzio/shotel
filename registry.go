package shotel

import (
	"sync"

	"github.com/zoobzio/capitan"
	"go.opentelemetry.io/otel/log"
)

// TransformerFunc converts a typed value to OTEL log attributes.
// Return nil or empty slice to skip the field.
type TransformerFunc[T any] func(key string, value T) []log.KeyValue

// transformerRegistry manages custom field transformers.
type transformerRegistry struct {
	mu           sync.RWMutex
	transformers map[capitan.Variant]func(capitan.Field) []log.KeyValue
}

// global registry for custom transformers.
var globalRegistry = &transformerRegistry{
	transformers: make(map[capitan.Variant]func(capitan.Field) []log.KeyValue),
}

// RegisterTransformer registers a custom transformer for a specific variant.
//
// The transformer receives the field key and typed value, returning OTEL log attributes.
// Type assertion is handled automatically - your function receives the concrete type.
//
// Example:
//
//	type OrderInfo struct {
//	    ID    string
//	    Total float64
//	    Secret string // Not logged
//	}
//
//	orderVariant := capitan.Variant("myapp.OrderInfo")
//
//	shotel.RegisterTransformer(orderVariant, func(key string, order OrderInfo) []log.KeyValue {
//	    return []log.KeyValue{
//	        log.String(key+".id", order.ID),
//	        log.Float64(key+".total", order.Total),
//	        // Secret field omitted
//	    }
//	})
func RegisterTransformer[T any](variant capitan.Variant, fn TransformerFunc[T]) {
	globalRegistry.mu.Lock()
	defer globalRegistry.mu.Unlock()

	// Wrap the typed transformer in a type-erased function
	globalRegistry.transformers[variant] = func(f capitan.Field) []log.KeyValue {
		// Type assert to GenericField[T]
		if gf, ok := f.(capitan.GenericField[T]); ok {
			return fn(f.Key().Name(), gf.Get())
		}
		return nil
	}
}

// UnregisterTransformer removes a custom transformer for a specific variant.
func UnregisterTransformer(variant capitan.Variant) {
	globalRegistry.mu.Lock()
	defer globalRegistry.mu.Unlock()
	delete(globalRegistry.transformers, variant)
}

// lookup finds a registered transformer for the given variant.
func (r *transformerRegistry) lookup(variant capitan.Variant) (func(capitan.Field) []log.KeyValue, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	transformer, ok := r.transformers[variant]
	return transformer, ok
}
