package telegram

import "github.com/easayliu/alist-aria2-download/internal/interfaces/telegram/types"

// Re-export constants from types package for backward compatibility
const (
	MaxDisplayItems          = types.MaxDisplayItems
	MaxSuggestions           = types.MaxSuggestions
	HighConfidence           = types.HighConfidence
	MediumConfidence         = types.MediumConfidence
	MessageAutoDeleteSeconds = types.MessageAutoDeleteSeconds
)
