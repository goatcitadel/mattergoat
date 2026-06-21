// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

// MatterGoat session RHS panel. Read-only governed view of a collaboration
// session: its state, participants, turn timeline, and pending approvals (which
// a permitted human can resolve here). Data comes from the views.mattergoat
// Redux slice, refreshed on the mg_* websocket events.

import React, {useEffect} from 'react';

import type {MGApproval, MGSession, MGTurn} from '@mattermost/types/mattergoat';

import {resolveMatterGoatApproval} from 'actions/mattergoat';

type Props = {
    selectedSessionId: string;
    session?: MGSession;
    turns: MGTurn[];
    approvals: MGApproval[];
    actions: {
        closeRightHandSide: () => void;
        fetchMatterGoatSessionDetail: (sessionId: string) => void;
    };
};

const sectionTitle: React.CSSProperties = {
    fontWeight: 600,
    fontSize: 12,
    textTransform: 'uppercase',
    letterSpacing: '0.02em',
    color: 'rgba(var(--center-channel-color-rgb), 0.56)',
    margin: '18px 16px 6px',
};

const row: React.CSSProperties = {
    padding: '6px 16px',
    borderBottom: '1px solid rgba(var(--center-channel-color-rgb), 0.08)',
    fontSize: 13,
};

const badge: React.CSSProperties = {
    display: 'inline-block',
    padding: '1px 6px',
    marginLeft: 6,
    borderRadius: 4,
    fontSize: 11,
    background: 'rgba(var(--center-channel-color-rgb), 0.08)',
    color: 'rgba(var(--center-channel-color-rgb), 0.75)',
};

export default function MatterGoatSessionRhs({selectedSessionId, session, turns, approvals, actions}: Props) {
    useEffect(() => {
        if (selectedSessionId) {
            actions.fetchMatterGoatSessionDetail(selectedSessionId);
        }
    }, [selectedSessionId]); // eslint-disable-line react-hooks/exhaustive-deps

    const onResolve = async (approvalId: string, approve: boolean) => {
        await resolveMatterGoatApproval(approvalId, {approve});
        if (selectedSessionId) {
            actions.fetchMatterGoatSessionDetail(selectedSessionId);
        }
    };

    return (
        <div
            className='mattergoat-session-rhs'
            style={{display: 'flex', flexDirection: 'column', flex: 1, overflowY: 'auto'}}
        >
            <div
                className='sidebar--right__header'
                style={{display: 'flex', alignItems: 'center', justifyContent: 'space-between', padding: '10px 16px'}}
            >
                <span style={{fontWeight: 600}}>{'MatterGoat session'}</span>
                <button
                    className='sidebar--right__close'
                    aria-label='Close'
                    onClick={actions.closeRightHandSide}
                    style={{background: 'none', border: 'none', cursor: 'pointer', color: 'inherit'}}
                >
                    <i className='icon icon-close'/>
                </button>
            </div>

            {!session && (
                <div style={{...row, color: 'rgba(var(--center-channel-color-rgb), 0.56)'}}>
                    {'No session selected.'}
                </div>
            )}

            {session && (
                <>
                    <div style={row}>
                        <div style={{fontWeight: 600}}>{session.title || 'Untitled session'}</div>
                        <div style={{marginTop: 2}}>
                            <span style={badge}>{session.state}</span>
                            <span style={badge}>{session.mode}</span>
                        </div>
                    </div>

                    <div style={sectionTitle}>{`Participants (${session.participants?.length ?? 0})`}</div>
                    {(session.participants ?? []).map((p) => (
                        <div
                            key={p.id}
                            style={row}
                        >
                            {p.agent_profile_id}
                            <span style={badge}>{p.role || 'agent'}</span>
                        </div>
                    ))}

                    <div style={sectionTitle}>{`Turns (${turns.length})`}</div>
                    {turns.map((t) => (
                        <div
                            key={t.id}
                            style={row}
                        >
                            {`#${t.turn_index + 1}`}
                            <span style={badge}>{t.status}</span>
                            {t.marker ? <span style={badge}>{t.marker}</span> : null}
                        </div>
                    ))}

                    <div style={sectionTitle}>{`Approvals (${approvals.length})`}</div>
                    {approvals.map((a) => (
                        <div
                            key={a.id}
                            style={row}
                        >
                            <div>
                                {a.action}
                                <span style={badge}>{a.risk_level}</span>
                                <span style={badge}>{a.status}</span>
                            </div>
                            {a.status === 'pending' && (
                                <div style={{marginTop: 6, display: 'flex', gap: 8}}>
                                    <button
                                        className='btn btn-primary btn-sm'
                                        onClick={() => onResolve(a.id, true)}
                                    >
                                        {'Approve'}
                                    </button>
                                    <button
                                        className='btn btn-tertiary btn-sm'
                                        onClick={() => onResolve(a.id, false)}
                                    >
                                        {'Reject'}
                                    </button>
                                </div>
                            )}
                        </div>
                    ))}
                </>
            )}
        </div>
    );
}
