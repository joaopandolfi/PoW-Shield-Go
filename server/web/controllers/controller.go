package controllers

import (
	"pow-shield-go/models"
	"pow-shield-go/web/server"
)

var SystemPermissions = []string{models.PermissionSystem}

// Controller public contract
type Controller interface {
	SetupRouter(s *server.Server)
}
