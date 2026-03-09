package config

import (
	"testing"

	"github.com/conductorone/baton-sdk/pkg/field"
	"github.com/stretchr/testify/assert"
)

func TestValidateConfig(t *testing.T) {
	tests := []struct {
		name    string
		config  *TwilioSegmentV2
		wantErr bool
	}{
		{
			name: "valid config",
			config: &TwilioSegmentV2{
				AccessToken: "test-access-token",
				BaseUrl:     "https://api.segmentapis.com",
			},
			wantErr: false,
		},
		{
			name: "valid config - only required fields",
			config: &TwilioSegmentV2{
				AccessToken: "test-access-token",
			},
			wantErr: false,
		},
		{
			name:    "invalid config - missing access token",
			config:  &TwilioSegmentV2{},
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
