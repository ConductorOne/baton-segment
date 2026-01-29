package main

import (
	cfg "github.com/conductorone/baton-segment/pkg/config"
	"github.com/conductorone/baton-sdk/pkg/config"
)

func main() {
	config.Generate("segment", cfg.Config)
}
