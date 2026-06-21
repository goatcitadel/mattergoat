// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';
import {useIntl} from 'react-intl';

import type {Post} from '@mattermost/types/posts';

import {Posts} from 'mattermost-redux/constants';

import MatterGoatAgentProvenance, {MG_AGENT_RESPONSE_POST_TYPE} from 'components/mattergoat/agent_provenance';
import PostBodyAdditionalContent from 'components/post_view/post_body_additional_content';
import PostMessageView from 'components/post_view/post_message_view';

import type {PluginsState} from 'types/store/plugins';

type Props = {
    id?: string;
    post: Post;
    isEmbedVisible?: boolean;
    pluginPostTypes?: PluginsState['postTypes'];
    isRHS: boolean;
    compactDisplay?: boolean;
    isChannelAutotranslated: boolean;
};

export default function MessageWithAdditionalContent({
    post,
    isEmbedVisible,
    pluginPostTypes,
    isRHS,
    compactDisplay,
    isChannelAutotranslated,
}: Props) {
    const hasPlugin = post.type && pluginPostTypes && Object.hasOwn(pluginPostTypes, post.type);
    const {locale} = useIntl();
    let msg;
    const messageWrapper = (
        <PostMessageView
            post={post}
            isRHS={isRHS}
            compactDisplay={compactDisplay}
            isChannelAutotranslated={isChannelAutotranslated}
            userLanguage={locale}
        />
    );
    if (post.state === Posts.POST_DELETED || hasPlugin) {
        msg = messageWrapper;
    } else {
        msg = (
            <PostBodyAdditionalContent
                post={post}
                isEmbedVisible={isEmbedVisible}
            >
                {messageWrapper}
            </PostBodyAdditionalContent>
        );
    }

    // MatterGoat: append a governed provenance strip below the message for
    // agent-authored posts. The component self-guards (renders null unless the
    // post carries mg_session_id), so this is inert for all other posts and
    // when the feature is disabled.
    if (post.type === MG_AGENT_RESPONSE_POST_TYPE) {
        return (
            <>
                {msg}
                <MatterGoatAgentProvenance post={post}/>
            </>
        );
    }

    return msg;
}
