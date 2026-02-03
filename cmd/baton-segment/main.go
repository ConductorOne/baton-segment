package main

import (
	"context"

	cfg "github.com/conductorone/baton-segment/pkg/config"
	"github.com/conductorone/baton-segment/pkg/connector"
	"github.com/conductorone/baton-sdk/pkg/config"
	"github.com/conductorone/baton-sdk/pkg/connectorrunner"
)

var version = "dev"

func main() {
	ctx := context.Background()
	config.RunConnector(ctx,
		"baton-segment",
		version,
		cfg.Config,
		connector.New,
		connectorrunner.WithSessionStoreEnabled(),
		connectorrunner.WithDefaultCapabilitiesConnectorBuilderV2(&connector.Segment{}),
	)
}
