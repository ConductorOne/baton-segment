package config

import (
	"github.com/conductorone/baton-sdk/pkg/field"
)

const (
	AccessTokenKey = "token"
	BaseURLKey     = "base-url"

	DefaultBaseURL = "https://api.segmentapis.com"
)

var (
	// AccessTokenField is the personal access token for Segment API authentication.
	AccessTokenField = field.StringField(
		AccessTokenKey,
		field.WithDisplayName("Access Token"),
		field.WithDescription("Personal Access Token (PAT) for Segment API authentication"),
		field.WithRequired(true),
		field.WithIsSecret(true),
	)

	// BaseURLField is the base URL for the Segment API.
	BaseURLField = field.StringField(
		BaseURLKey,
		field.WithDisplayName("Base URL"),
		field.WithDescription("Base URL for the Segment API"),
		field.WithDefaultValue(DefaultBaseURL),
		field.WithHidden(true),
		field.WithExportTarget(field.ExportTargetCLIOnly),
	)

	ConfigurationFields = []field.SchemaField{
		AccessTokenField,
		BaseURLField,
	}
)

//go:generate go run -tags=generate ./gen
var Config = field.NewConfiguration(
	ConfigurationFields,
	field.WithConnectorDisplayName("Twilio Segment"),
	field.WithHelpUrl("/docs/baton/segment"),
	field.WithIconUrl("/static/app-icons/segment.svg"),
)
