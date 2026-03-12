package main

import (
	"github.com/conductorone/baton-sdk/pkg/config"
	cfg "github.com/conductorone/baton-segment/pkg/config"
)

func main() {
	config.Generate("segment", cfg.Config)
}
