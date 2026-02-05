package engine

// Generator defines the interface for the code generation engine.
// It allows commands to work with any implementation that can generate
// code from template data.
type Generator interface {
	Generate(data Data) error
}

// Compile-time check that Core implements Generator.
var _ Generator = (*Core)(nil)
