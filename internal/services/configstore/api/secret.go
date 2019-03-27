// Copyright 2019 Sorint.lab
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied
// See the License for the specific language governing permissions and
// limitations under the License.

package api

import (
	"encoding/json"
	"net/http"

	"github.com/sorintlab/agola/internal/db"
	"github.com/sorintlab/agola/internal/services/configstore/command"
	"github.com/sorintlab/agola/internal/services/configstore/readdb"
	"github.com/sorintlab/agola/internal/services/types"

	"github.com/gorilla/mux"
	"go.uber.org/zap"
)

type SecretHandler struct {
	log    *zap.SugaredLogger
	readDB *readdb.ReadDB
}

func NewSecretHandler(logger *zap.Logger, readDB *readdb.ReadDB) *SecretHandler {
	return &SecretHandler{log: logger.Sugar(), readDB: readDB}
}

func (h *SecretHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	secretID := vars["secretid"]

	var secret *types.Secret
	err := h.readDB.Do(func(tx *db.Tx) error {
		var err error
		secret, err = h.readDB.GetSecretByID(tx, secretID)
		return err
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if secret == nil {
		http.Error(w, "", http.StatusNotFound)
		return
	}

	if err := json.NewEncoder(w).Encode(secret); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

type SecretsHandler struct {
	log    *zap.SugaredLogger
	readDB *readdb.ReadDB
}

func NewSecretsHandler(logger *zap.Logger, readDB *readdb.ReadDB) *SecretsHandler {
	return &SecretsHandler{log: logger.Sugar(), readDB: readDB}
}

func (h *SecretsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()

	_, tree := query["tree"]

	parentType, parentRef, err := GetConfigTypeRef(r)
	if httpError(w, err) {
		h.log.Errorf("err: %+v", err)
		return
	}

	var secrets []*types.Secret
	err = h.readDB.Do(func(tx *db.Tx) error {
		parentID, err := h.readDB.ResolveConfigID(tx, parentType, parentRef)
		if err != nil {
			return err
		}
		if tree {
			secrets, err = h.readDB.GetSecretsTree(tx, parentType, parentID)
		} else {
			secrets, err = h.readDB.GetSecrets(tx, parentID)
		}
		// populate parent path
		for _, s := range secrets {
			pp, err := h.readDB.GetParentPath(tx, s.Parent.Type, s.Parent.ID)
			if err != nil {
				return err
			}
			s.Parent.Path = pp
		}
		return err
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := json.NewEncoder(w).Encode(secrets); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

type CreateSecretHandler struct {
	log    *zap.SugaredLogger
	ch     *command.CommandHandler
	readDB *readdb.ReadDB
}

func NewCreateSecretHandler(logger *zap.Logger, ch *command.CommandHandler) *CreateSecretHandler {
	return &CreateSecretHandler{log: logger.Sugar(), ch: ch}
}

func (h *CreateSecretHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	parentType, parentRef, err := GetConfigTypeRef(r)
	if httpError(w, err) {
		h.log.Errorf("err: %+v", err)
		return
	}

	var secret *types.Secret
	d := json.NewDecoder(r.Body)
	if err := d.Decode(&secret); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	secret.Parent.Type = parentType
	secret.Parent.ID = parentRef

	secret, err = h.ch.CreateSecret(ctx, secret)
	if httpError(w, err) {
		h.log.Errorf("err: %+v", err)
		return
	}

	if err := json.NewEncoder(w).Encode(secret); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

type DeleteSecretHandler struct {
	log *zap.SugaredLogger
	ch  *command.CommandHandler
}

func NewDeleteSecretHandler(logger *zap.Logger, ch *command.CommandHandler) *DeleteSecretHandler {
	return &DeleteSecretHandler{log: logger.Sugar(), ch: ch}
}

func (h *DeleteSecretHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	vars := mux.Vars(r)
	secretName := vars["secretname"]

	parentType, parentRef, err := GetConfigTypeRef(r)
	if httpError(w, err) {
		h.log.Errorf("err: %+v", err)
		return
	}

	err = h.ch.DeleteSecret(ctx, parentType, parentRef, secretName)
	if httpError(w, err) {
		h.log.Errorf("err: %+v", err)
	}
}