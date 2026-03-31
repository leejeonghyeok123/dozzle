package deploy

import (
	"testing"

	"github.com/amir20/dozzle/internal/container"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_authenticatedRepoURL(t *testing.T) {
	result := authenticatedRepoURL("https://github.com/acme/app.git", "bot", "secret")
	assert.Equal(t, "https://bot:secret@github.com/acme/app.git", result)
}

func Test_resolveRequest_usesLabels(t *testing.T) {
	c := container.Container{
		ID: "abc",
		Labels: map[string]string{
			labelEnabled: "true",
			labelPath:    "/opt/apps/app",
			labelRepo:    "https://github.com/acme/app.git",
			labelBranch:  "main",
			labelCompose: "docker-compose.yml",
			labelService: "web",
		},
	}
	req, err := resolveRequest(c, Request{})
	require.NoError(t, err)
	assert.Equal(t, "/opt/apps/app", req.ProjectPath)
	assert.Equal(t, "https://github.com/acme/app.git", req.RepoURL)
	assert.Equal(t, "main", req.Branch)
	assert.Equal(t, "docker-compose.yml", req.ComposeFile)
	assert.Equal(t, "web", req.Service)
}

func Test_resolveRequest_requiresEnabledLabel(t *testing.T) {
	c := container.Container{
		ID:     "abc",
		Labels: map[string]string{labelPath: "/opt/apps/app"},
	}
	_, err := resolveRequest(c, Request{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "deploy is not enabled")
}

