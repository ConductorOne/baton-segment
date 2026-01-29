package config

import (
	"github.com/conductorone/baton-sdk/pkg/field"
)

var (
	TokenField = field.StringField(
		"token",
		field.WithDisplayName("Segment API token"),
		field.WithDescription("The API token used to authenticate with the Segment API."),
		field.WithIsSecret(true),
		field.WithRequired(true),
	)
)

var (
	ConfigurationFields = []field.SchemaField{
		TokenField,
	}

	FieldRelationships = []field.SchemaFieldRelationship{}
)

//go:generate go run ./gen
var Config = field.NewConfiguration(
	ConfigurationFields,
	field.WithConstraints(FieldRelationships...),
	field.WithConnectorDisplayName("Segment"),
	field.WithHelpUrl("/docs/baton/segment"),
	field.WithIconUrl("/static/app-icons/segment.svg"),
)
