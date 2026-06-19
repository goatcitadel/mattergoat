// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

// MatterGoatAgentProvenance renders a compact provenance strip for a governed
// agent post (post.type === "custom_mg_agent_response"). It reads the structured
// metadata the orchestrator stamps onto post.props — never the message body — so
// users can see which agent/provider produced a message, its protocol marker,
// confidence, and whether it is the final synthesis.
//
// Self-contained presentational component. To surface it, render
// <MatterGoatAgentProvenance post={post}/> from the post body for the
// custom_mg_agent_response post type (e.g. via a guarded branch in
// post_message_view, or a plugin postType component).

import React from 'react';

import type {Post} from '@mattermost/types/posts';

type Props = {
    post: Post;
};

const badgeStyle: React.CSSProperties = {
    display: 'inline-block',
    padding: '1px 6px',
    marginRight: 6,
    marginBottom: 2,
    borderRadius: 4,
    fontSize: 11,
    lineHeight: '16px',
    background: 'rgba(var(--center-channel-color-rgb), 0.08)',
    color: 'rgba(var(--center-channel-color-rgb), 0.75)',
};

const finalBadgeStyle: React.CSSProperties = {
    ...badgeStyle,
    background: 'rgba(var(--denim-button-bg-rgb), 0.12)',
    color: 'var(--denim-button-bg)',
    fontWeight: 600,
};

function str(value: unknown): string {
    return typeof value === 'string' ? value : '';
}

export default function MatterGoatAgentProvenance({post}: Props): JSX.Element | null {
    const props = (post.props || {}) as Record<string, unknown>;

    const provider = str(props.mg_provider);
    const model = str(props.mg_model);
    const marker = str(props.mg_marker);
    const confidence = str(props.mg_confidence);
    const isFinal = Boolean(props.mg_final_synthesis);
    const sessionId = str(props.mg_session_id);

    if (!sessionId) {
        return null;
    }

    return (
        <div className='mattergoat-agent-provenance' style={{marginTop: 4}}>
            <span style={badgeStyle}>{'MatterGoat agent'}</span>
            {provider ? <span style={badgeStyle}>{`provider: ${provider}`}</span> : null}
            {model ? <span style={badgeStyle}>{`model: ${model}`}</span> : null}
            {confidence ? <span style={badgeStyle}>{`confidence: ${confidence}`}</span> : null}
            {marker ? <span style={badgeStyle}>{marker}</span> : null}
            {isFinal ? <span style={finalBadgeStyle}>{'final synthesis'}</span> : null}
        </div>
    );
}
