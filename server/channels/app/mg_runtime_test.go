// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/shared/request"
)

func TestGoatCitadelRuntimeComplete(t *testing.T) {
	t.Run("maps request and response over the documented contract", func(t *testing.T) {
		var gotReq goatCitadelTurnRequest
		var gotAuth, gotIdem, gotPath string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotAuth = r.Header.Get("Authorization")
			gotIdem = r.Header.Get("Idempotency-Key")
			gotPath = r.URL.Path
			_ = json.NewDecoder(r.Body).Decode(&gotReq)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"message":"hi","markers":["HANDOFF_COMPLETE"],"needs_approval":true,"provider":"openai","model":"gpt","run_id":"run_1"}`))
		}))
		defer server.Close()

		rt := &goatCitadelRuntime{endpoint: server.URL, token: "secret", client: server.Client()}
		res, err := rt.Complete(request.TestContext(t), MGRuntimeRequest{
			SessionID: "s1", TurnID: "t1", AgentRef: "a1", UserID: "u1", ChannelID: "c1",
			Messages: []BridgeMessage{{Role: "user", AuthorRef: "u1", Message: "hello"}},
		})
		require.NoError(t, err)

		// Response → structured result.
		assert.Equal(t, "hi", res.Message)
		assert.Equal(t, []string{"HANDOFF_COMPLETE"}, res.Markers)
		assert.True(t, res.NeedsApproval)
		assert.Equal(t, "openai", res.Provider)
		assert.Equal(t, "gpt", res.Model)
		assert.Equal(t, "run_1", res.RunID)

		// Request headers + body.
		assert.Equal(t, "/api/v1/turns/complete", gotPath)
		assert.Equal(t, "Bearer secret", gotAuth)
		assert.Equal(t, "t1", gotIdem) // idempotency key is the turn id
		assert.Equal(t, "s1", gotReq.SessionID)
		assert.Equal(t, "u1", gotReq.UserRef)
		assert.Equal(t, "c1", gotReq.ChannelRef)
		assert.Equal(t, mgClientOperation, gotReq.Operation) // defaulted when empty
		require.Len(t, gotReq.Messages, 1)
		assert.Equal(t, "u1", gotReq.Messages[0].AuthorRef) // identity is structured, not a name: prefix
	})

	t.Run("surfaces the error envelope on non-200", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"error":{"code":"forbidden","message":"nope"}}`))
		}))
		defer server.Close()

		rt := &goatCitadelRuntime{endpoint: server.URL, token: "x", client: server.Client()}
		_, err := rt.Complete(request.TestContext(t), MGRuntimeRequest{TurnID: "t2"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "403")
		assert.Contains(t, err.Error(), "nope")
	})

	t.Run("fails clearly when unconfigured", func(t *testing.T) {
		rt := &goatCitadelRuntime{}
		_, err := rt.Complete(request.TestContext(t), MGRuntimeRequest{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not configured")
	})
}
