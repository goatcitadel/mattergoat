// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package slashcommands

import (
	"fmt"
	"strings"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/i18n"
	"github.com/mattermost/mattermost/server/public/shared/request"
	"github.com/mattermost/mattermost/server/v8/channels/app"
)

type GoatProvider struct{}

const CmdGoat = "goat"

func init() {
	app.RegisterCommandProvider(&GoatProvider{})
}

func (*GoatProvider) GetTrigger() string {
	return CmdGoat
}

func (*GoatProvider) GetCommand(a *app.App, T i18n.TranslateFunc) *model.Command {
	return &model.Command{
		Trigger:          CmdGoat,
		AutoComplete:     true,
		AutoCompleteDesc: T("api.command_goat.desc"),
		AutoCompleteHint: T("api.command_goat.hint"),
		DisplayName:      T("api.command_goat.name"),
	}
}

func goatResponse(text string) *model.CommandResponse {
	return &model.CommandResponse{ResponseType: model.CommandResponseTypeEphemeral, Text: text}
}

func (p *GoatProvider) DoCommand(a *app.App, rctx request.CTX, args *model.CommandArgs, message string) *model.CommandResponse {
	if !a.MatterGoatEnabled() {
		return goatResponse("MatterGoat AI collaboration is not enabled on this server.")
	}

	fields := strings.SplitN(strings.TrimSpace(message), " ", 2)
	sub := strings.ToLower(fields[0])
	rest := ""
	if len(fields) > 1 {
		rest = strings.TrimSpace(fields[1])
	}

	switch sub {
	case "ask", "debate", "review":
		return p.doAsk(a, rctx, args, rest)
	case "synthesize":
		return p.doSynthesize(a, rctx, args)
	case "export":
		return p.doExport(a, rctx, args)
	case "approvals":
		return p.doApprovals(a, rctx, args)
	case "status":
		return p.doStatus(a, rctx, args)
	default:
		return goatResponse(goatHelp())
	}
}

func goatHelp() string {
	return strings.Join([]string{
		"**MatterGoat** — multi-agent AI collaboration:",
		"- `/goat ask <agent1,agent2> <task>` — start a session and let the agents work the task",
		"- `/goat debate <agent1,agent2> <question>` — alias of ask",
		"- `/goat synthesize` — produce a final synthesis for this thread's session",
		"- `/goat export` — export this thread's session to AMCL Markdown",
		"- `/goat approvals` — list pending approvals",
		"- `/goat status` — show MatterGoat sessions in this channel",
		"Agents are referenced by registered agent-profile id or display name.",
	}, "\n")
}

// resolveAgents maps comma-separated identifiers (id or display name) to profile ids.
func (p *GoatProvider) resolveAgents(a *app.App, csv string) ([]string, []string) {
	all, err := a.MGGetAgentProfilesByOwner("", "")
	if err != nil {
		return nil, nil
	}
	byID := map[string]*model.MGAgentProfile{}
	byName := map[string]*model.MGAgentProfile{}
	for _, prof := range all {
		byID[prof.Id] = prof
		byName[strings.ToLower(prof.DisplayName)] = prof
	}
	var ids, unresolved []string
	for _, tok := range strings.Split(csv, ",") {
		tok = strings.TrimSpace(tok)
		if tok == "" {
			continue
		}
		if prof, ok := byID[tok]; ok {
			ids = append(ids, prof.Id)
		} else if prof, ok := byName[strings.ToLower(tok)]; ok {
			ids = append(ids, prof.Id)
		} else {
			unresolved = append(unresolved, tok)
		}
	}
	return ids, unresolved
}

func (p *GoatProvider) doAsk(a *app.App, rctx request.CTX, args *model.CommandArgs, rest string) *model.CommandResponse {
	parts := strings.SplitN(rest, " ", 2)
	if len(parts) < 2 {
		return goatResponse("Usage: `/goat ask <agent1,agent2> <task>`")
	}
	ids, unresolved := p.resolveAgents(a, parts[0])
	if len(ids) == 0 {
		return goatResponse("No matching agents found. Register agent profiles first, or check the names/ids.")
	}
	task := strings.TrimSpace(parts[1])

	// Anchor the session to a thread: reuse the current thread root, or post the
	// task as the user so agents have a visible root to reply under.
	rootID := args.RootId
	if rootID == "" {
		channel, cErr := a.GetChannel(rctx, args.ChannelId)
		if cErr != nil {
			return goatResponse("Could not access this channel.")
		}
		taskPost := &model.Post{UserId: args.UserId, ChannelId: args.ChannelId, Message: task}
		saved, _, pErr := a.CreatePost(rctx, taskPost, channel, model.CreatePostFlags{})
		if pErr != nil {
			return goatResponse("Could not post the task message.")
		}
		rootID = saved.Id
	}

	session, err := a.MGStartSession(rctx, &model.MGStartSessionRequest{
		Title:           task,
		ChannelId:       args.ChannelId,
		RootPostId:      rootID,
		Mode:            model.MGModeStrictTurns,
		AgentProfileIds: ids,
	})
	if err != nil {
		return goatResponse("Failed to start session: " + err.Message)
	}

	if runErr := a.MGRunSession(rctx, session.Id); runErr != nil {
		return goatResponse(fmt.Sprintf("Session `%s` started, but a turn failed: %s", session.Id, runErr.Message))
	}

	msg := fmt.Sprintf("Started MatterGoat session `%s` with %d agent(s).", session.Id, len(ids))
	if len(unresolved) > 0 {
		msg += " Unresolved agents: " + strings.Join(unresolved, ", ")
	}
	return goatResponse(msg)
}

func (p *GoatProvider) latestSession(a *app.App, args *model.CommandArgs) *model.MGSession {
	sessions, err := a.MGGetSessionsForChannel(args.ChannelId)
	if err != nil || len(sessions) == 0 {
		return nil
	}
	if args.RootId != "" {
		for _, s := range sessions {
			if s.RootPostId == args.RootId {
				return s
			}
		}
	}
	return sessions[0]
}

func (p *GoatProvider) doSynthesize(a *app.App, rctx request.CTX, args *model.CommandArgs) *model.CommandResponse {
	session := p.latestSession(a, args)
	if session == nil {
		return goatResponse("No MatterGoat session found in this channel.")
	}
	if err := a.MGSynthesize(rctx, session.Id); err != nil {
		return goatResponse("Synthesis failed: " + err.Message)
	}
	return goatResponse("Final synthesis posted.")
}

func (p *GoatProvider) doExport(a *app.App, rctx request.CTX, args *model.CommandArgs) *model.CommandResponse {
	session := p.latestSession(a, args)
	if session == nil {
		return goatResponse("No MatterGoat session found in this channel.")
	}
	if _, err := a.MGExportMarkdown(rctx, session.Id); err != nil {
		return goatResponse("Export failed: " + err.Message)
	}
	return goatResponse("Session exported to AMCL Markdown (posted in the thread).")
}

func (p *GoatProvider) doApprovals(a *app.App, rctx request.CTX, args *model.CommandArgs) *model.CommandResponse {
	session := p.latestSession(a, args)
	if session == nil {
		return goatResponse("No MatterGoat session found in this channel.")
	}
	approvals, err := a.MGGetApprovalsForSession(session.Id)
	if err != nil {
		return goatResponse("Could not load approvals.")
	}
	var pending []string
	for _, ap := range approvals {
		if ap.Status == model.MGApprovalStatusPending {
			pending = append(pending, fmt.Sprintf("- `%s` %s (risk: %s)", ap.Id, ap.Action, ap.RiskLevel))
		}
	}
	if len(pending) == 0 {
		return goatResponse("No pending approvals.")
	}
	return goatResponse("Pending approvals:\n" + strings.Join(pending, "\n"))
}

func (p *GoatProvider) doStatus(a *app.App, rctx request.CTX, args *model.CommandArgs) *model.CommandResponse {
	sessions, err := a.MGGetSessionsForChannel(args.ChannelId)
	if err != nil {
		return goatResponse("Could not load sessions.")
	}
	if len(sessions) == 0 {
		return goatResponse("No MatterGoat sessions in this channel yet.")
	}
	var lines []string
	for _, s := range sessions {
		title := s.Title
		if title == "" {
			title = "(untitled)"
		}
		lines = append(lines, fmt.Sprintf("- `%s` — %s — state: **%s**", s.Id, title, s.State))
	}
	return goatResponse("MatterGoat sessions:\n" + strings.Join(lines, "\n"))
}
