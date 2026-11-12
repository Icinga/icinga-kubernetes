package daemon

import "testing"

func TestNamespacesConfigValidate(t *testing.T) {
	tests := map[string]struct {
		config  NamespacesConfig
		wantErr bool
	}{
		"Empty":       {NamespacesConfig{}, false},
		"Include":     {NamespacesConfig{Include: []string{"a", "b"}}, false},
		"Exclude":     {NamespacesConfig{Exclude: []string{"a", "b"}}, false},
		"Both":        {NamespacesConfig{Include: []string{"a"}, Exclude: []string{"b"}}, true},
		"EmptyList":   {NamespacesConfig{Include: []string{}, Exclude: []string{"b"}}, false},
		"EmptyName":   {NamespacesConfig{Include: []string{""}}, true},
		"InvalidName": {NamespacesConfig{Exclude: []string{"Production"}}, true},
		"Duplicate":   {NamespacesConfig{Exclude: []string{"a", "a"}}, true},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			if err := tt.config.Validate(); (err != nil) != tt.wantErr {
				t.Errorf("Validate() = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
