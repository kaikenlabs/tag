package commands

import "gitlab.com/Vitrifi/tag/internal/engine"

type Generator interface {
	Generate(data engine.Data) error
}
