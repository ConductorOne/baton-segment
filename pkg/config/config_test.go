package config

import (
	"testing"

	"github.com/conductorone/baton-sdk/pkg/field"
	"github.com/stretchr/testify/assert"
)

func TestValidateConfig(t *testing.T) {
	tests := []struct {
		name    string
		config  *Segment
		wantErr bool
	}{
		{
			name: "valid config",
			config: &Segment{
				Token: "test-access-token",
				BaseUrl:     "https://api.segmentapis.com",
			},
			wantErr: false,
		},
		{
			name: "valid config - only required fields",
			config: &Segment{
				Token: "test-access-token",
			},
			wantErr: false,
		},
		{
			name:    "invalid config - missing access token",
			config:  &Segment{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := field.Validate(Config, tt.config)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
