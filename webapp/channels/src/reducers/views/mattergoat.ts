// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

// MatterGoat multi-agent AI collaboration view state.
//
// Holds the data the RHS session panel renders: the channel's sessions list,
// the focused session id, and per-session detail (turns, approvals). Detail is
// refreshed both by explicit fetches and by the mg_* websocket events the
// orchestrator publishes (see actions/websocket_actions).

import {combineReducers} from 'redux';

import type {MGApproval, MGSession, MGTurn} from '@mattermost/types/mattergoat';

import {UserTypes} from 'mattermost-redux/action_types';

import {ActionTypes} from 'utils/constants';

import type {MMAction} from 'types/store';

function selectedSessionId(state = '', action: MMAction): string {
    switch (action.type) {
    case ActionTypes.SELECTED_MATTERGOAT_SESSION:
        return action.sessionId;
    case UserTypes.LOGOUT_SUCCESS:
        return '';
    default:
        return state;
    }
}

function sessionsByChannel(state: {[channelId: string]: MGSession[]} = {}, action: MMAction) {
    switch (action.type) {
    case ActionTypes.RECEIVED_MATTERGOAT_CHANNEL_SESSIONS:
        return {...state, [action.channelId]: action.sessions};
    case UserTypes.LOGOUT_SUCCESS:
        return {};
    default:
        return state;
    }
}

function sessions(state: {[sessionId: string]: MGSession} = {}, action: MMAction) {
    switch (action.type) {
    case ActionTypes.RECEIVED_MATTERGOAT_SESSION_DETAIL:
        return {...state, [action.session.id]: action.session};
    case UserTypes.LOGOUT_SUCCESS:
        return {};
    default:
        return state;
    }
}

function turnsBySession(state: {[sessionId: string]: MGTurn[]} = {}, action: MMAction) {
    switch (action.type) {
    case ActionTypes.RECEIVED_MATTERGOAT_SESSION_DETAIL:
        return {...state, [action.session.id]: action.turns};
    case UserTypes.LOGOUT_SUCCESS:
        return {};
    default:
        return state;
    }
}

function approvalsBySession(state: {[sessionId: string]: MGApproval[]} = {}, action: MMAction) {
    switch (action.type) {
    case ActionTypes.RECEIVED_MATTERGOAT_SESSION_DETAIL:
        return {...state, [action.session.id]: action.approvals};
    case UserTypes.LOGOUT_SUCCESS:
        return {};
    default:
        return state;
    }
}

export default combineReducers({
    selectedSessionId,
    sessionsByChannel,
    sessions,
    turnsBySession,
    approvalsBySession,
});
