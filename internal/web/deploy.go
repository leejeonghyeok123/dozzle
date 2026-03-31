package web

import (
	"encoding/json"
	"net/http"
	"os"
	"strconv"

	"github.com/amir20/dozzle/internal/auth"
	"github.com/amir20/dozzle/internal/container"
	"github.com/amir20/dozzle/internal/deploy"
	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog/log"
)

type deployRequest struct {
	ProjectPath string `json:"projectPath"`
	RepoURL     string `json:"repoUrl"`
	Branch      string `json:"branch"`
	ComposeFile string `json:"composeFile"`
	Service     string `json:"service"`
	GitUsername string `json:"gitUsername"`
	GitToken    string `json:"gitToken"`
	Bootstrap   bool   `json:"bootstrap"`
}

type deployCredentialRequest struct {
	GitUsername string `json:"gitUsername"`
	GitToken    string `json:"gitToken"`
}

func (h *handler) deployContainer(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	userLabels, permit, requestedBy := h.actionAuth(r)
	if !permit {
		http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
		return
	}

	containerService, err := h.hostService.FindContainer(hostKey(r), id, userLabels)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	var input deployRequest
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if input.GitToken == "" {
		if cred, ok := h.deployCredentialStore().Get(hostKey(r)); ok {
			input.GitToken = cred.Token
			if input.GitUsername == "" {
				input.GitUsername = cred.Username
			}
		}
	}

	runID, err := containerService.Deploy(r.Context(), deploy.Request{
		ContainerID: containerService.Container.ID,
		ProjectPath: input.ProjectPath,
		RepoURL:     input.RepoURL,
		Branch:      input.Branch,
		ComposeFile: input.ComposeFile,
		Service:     input.Service,
		GitUsername: input.GitUsername,
		GitToken:    input.GitToken,
		Bootstrap:   input.Bootstrap,
		RequestedBy: requestedBy,
		AllowDisabled: true,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	writeJSON(w, http.StatusAccepted, map[string]string{"runId": runID})
}

func (h *handler) deployStatus(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	runID := chi.URLParam(r, "runId")
	userLabels, permit, _ := h.actionAuth(r)
	if !permit {
		http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
		return
	}
	containerService, err := h.hostService.FindContainer(hostKey(r), id, userLabels)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	status, err := containerService.DeployStatus(r.Context(), runID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, status)
}

func (h *handler) deployLogs(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	runID := chi.URLParam(r, "runId")
	userLabels, permit, _ := h.actionAuth(r)
	if !permit {
		http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
		return
	}
	containerService, err := h.hostService.FindContainer(hostKey(r), id, userLabels)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	offset := 0
	if raw := r.URL.Query().Get("offset"); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil {
			http.Error(w, "invalid offset", http.StatusBadRequest)
			return
		}
		offset = value
	}
	chunk, err := containerService.DeployLogs(r.Context(), runID, offset)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, chunk)
}

func (h *handler) deployHistory(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	userLabels, permit, _ := h.actionAuth(r)
	if !permit {
		http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
		return
	}
	containerService, err := h.hostService.FindContainer(hostKey(r), id, userLabels)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	limit := 20
	if raw := r.URL.Query().Get("limit"); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil {
			http.Error(w, "invalid limit", http.StatusBadRequest)
			return
		}
		limit = value
	}
	items, err := containerService.DeployRecent(r.Context(), limit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *handler) saveDeployCredentials(w http.ResponseWriter, r *http.Request) {
	_, permit, _ := h.actionAuth(r)
	if !permit {
		http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
		return
	}
	var input deployCredentialRequest
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if input.GitToken == "" {
		http.Error(w, "gitToken is required", http.StatusBadRequest)
		return
	}
	if err := h.deployCredentialStore().Save(deploy.Credential{
		Host:     hostKey(r),
		Username: input.GitUsername,
		Token:    input.GitToken,
	}); err != nil {
		log.Error().Err(err).Msg("failed to save deploy credentials")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Error(w, "", http.StatusNoContent)
}

func (h *handler) actionAuth(r *http.Request) (container.ContainerLabels, bool, string) {
	userLabels := h.config.Labels
	permit := true
	requestedBy := "anonymous"
	if h.config.Authorization.Provider != NONE {
		user := auth.UserFromContext(r.Context())
		if user.ContainerLabels.Exists() {
			userLabels = user.ContainerLabels
		}
		permit = user.Roles.Has(auth.Actions)
		requestedBy = user.Username
	}
	return userLabels, permit, requestedBy
}

func (h *handler) deployCredentialStore() *deploy.CredentialStore {
	secret := os.Getenv("DOZZLE_DEPLOY_SECRET_KEY")
	if secret == "" {
		secret = "dozzle-default-deploy-secret-change-me"
	}
	return deploy.NewCredentialStore("./data/deploy_credentials.enc", secret)
}

