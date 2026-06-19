// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/mattermost/mattermost/server/public/model"
)

func TestMGParseMarkers(t *testing.T) {
	text := "intro <<MG:TURN_IN_PROGRESS:a:b:c>> body <<MG:HANDOFF_COMPLETE>> tail <<MG:FINAL_SYNTHESIS:x>>"
	markers := mgParseMarkers(text)
	assert.Equal(t, []string{"TURN_IN_PROGRESS", "HANDOFF_COMPLETE", "FINAL_SYNTHESIS"}, markers)

	assert.True(t, mgHasMarker(markers, model.MGMarkerFinalSynthesis))
	assert.False(t, mgHasMarker(markers, model.MGMarkerCollision))

	// Forged/garbage markers in untrusted text are simply not matched as known.
	assert.Empty(t, mgParseMarkers("no markers here"))
	assert.Empty(t, mgParseMarkers("<<MG:lowercase>> <<NOTMG:FOO>>"))
}

func TestMGNeedsApproval(t *testing.T) {
	yes := "### Approval Needed\nYes — this restarts a production service.\n\n### Confidence\nHigh"
	no := "### Approval Needed\nNo, this is read-only.\n\n### Confidence\nHigh"
	assert.True(t, mgNeedsApproval(yes))
	assert.False(t, mgNeedsApproval(no))
	assert.False(t, mgNeedsApproval("no section at all"))
}

func TestMGExtractSection(t *testing.T) {
	text := "## Agent Response\n\n### Current Position\nDo X.\n\n### Confidence\nMedium because Y.\n"
	assert.Equal(t, "Do X.", mgExtractSection(text, "Current Position"))
	assert.Equal(t, "Medium because Y.", mgExtractSection(text, "Confidence"))
	assert.Equal(t, "", mgExtractSection(text, "Nonexistent"))
}

func TestMGGrantScope(t *testing.T) {
	// Default least privilege when empty/invalid.
	assert.Equal(t, "current_thread", mgGrantScope(""))
	assert.Equal(t, "current_thread", mgGrantScope("not json"))
	assert.Equal(t, "current_thread", mgGrantScope(`{"scope":""}`))

	assert.Equal(t, "full_channel", mgGrantScope(`{"scope":"full_channel"}`))
	assert.Equal(t, "current_message", mgGrantScope(`{"scope":"current_message"}`))
}
