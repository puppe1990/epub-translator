package handlers

// The hosted catalog also lists retired models that return 404. Keep the
// selection focused on current text models, without claiming availability.
func providerModelOptions(slug, selected string) []string {
	if slug != "nvidia" {
		return nil
	}
	options := []string{
		"minimaxai/minimax-m3.1-flash",
		"google/gemma-4-31b-it",
		"nvidia/nemotron-3.5-lightning-30b-a3b",
		"nvidia/nemotron-3.5-super-vl-120b-a12b",
		"nvidia/nemotron-3-super-120b-a12b",
		"mistralai/mistral-large-4-1t-a52b",
	}
	for _, model := range options {
		if model == selected {
			return options
		}
	}
	// Never silently replace a previously saved model outside this list.
	if selected != "" {
		options = append([]string{selected}, options...)
	}
	return options
}
