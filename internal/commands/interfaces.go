package commands

import "github.com/kaikenlabs/tag/internal/engine"

type Generator interface {
	Generate(data engine.Data) error
}
