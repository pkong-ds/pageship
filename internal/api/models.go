package api

import (
	"github.com/oursky/pageship/internal/models"
)

type APIApp struct {
	*models.App
	URL string `json:"url"`
}

type APISite struct {
	*models.Site
	URL            string  `json:"url"`
	DeploymentName *string `json:"deploymentName"`
}

type APIDeployment struct {
	*models.Deployment
	SiteName *string `json:"siteName"`
	URL      *string `json:"url"`
}

type APIUser struct {
	ID          string                `json:"id"`
	Name        string                `json:"name"`
	Credentials []models.CredentialID `json:"credentials"`
}

type SitePatchRequest struct {
	DeploymentName *string `json:"deploymentName,omitempty"`
}
