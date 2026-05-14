package model

import (
	"fmt"
	"strings"
)

type Route struct {
	Alias    string
	Provider string
	Model    string
}

func Resolve(requestedModel string, aliases map[string]string) (Route, error) {
	requestedModel = strings.TrimSpace(requestedModel)
	if requestedModel == "" {
		return Route{}, fmt.Errorf("missing model")
	}

	if target, ok := aliases[requestedModel]; ok {
		return parseTarget(requestedModel, target)
	}

	lower := strings.ToLower(requestedModel)
	for _, alias := range []string{"opus", "sonnet", "haiku"} {
		if strings.Contains(lower, alias) {
			if target, ok := aliases[alias]; ok {
				return parseTarget(alias, target)
			}
		}
	}

	if strings.Contains(requestedModel, ":") {
		return parseTarget("", requestedModel)
	}

	return Route{}, fmt.Errorf("no alias configured for model %q", requestedModel)
}

func parseTarget(alias string, target string) (Route, error) {
	provider, modelName, ok := strings.Cut(strings.TrimSpace(target), ":")
	provider = strings.TrimSpace(provider)
	modelName = strings.TrimSpace(modelName)
	if !ok || provider == "" || modelName == "" {
		return Route{}, fmt.Errorf("invalid alias target %q", target)
	}
	return Route{
		Alias:    alias,
		Provider: provider,
		Model:    modelName,
	}, nil
}
