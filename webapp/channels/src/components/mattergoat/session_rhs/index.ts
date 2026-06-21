// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {connect} from 'react-redux';
import {bindActionCreators} from 'redux';
import type {Dispatch} from 'redux';

import {fetchMatterGoatSessionDetail} from 'actions/mattergoat';
import {closeRightHandSide} from 'actions/views/rhs';

import type {GlobalState} from 'types/store';

import MatterGoatSessionRhs from './session_rhs';

function mapStateToProps(state: GlobalState) {
    const mg = state.views.mattergoat;
    const selectedSessionId = mg.selectedSessionId;
    return {
        selectedSessionId,
        session: mg.sessions[selectedSessionId],
        turns: mg.turnsBySession[selectedSessionId] ?? [],
        approvals: mg.approvalsBySession[selectedSessionId] ?? [],
    };
}

function mapDispatchToProps(dispatch: Dispatch) {
    return {
        actions: bindActionCreators({
            closeRightHandSide,
            fetchMatterGoatSessionDetail,
        }, dispatch),
    };
}

export default connect(mapStateToProps, mapDispatchToProps)(MatterGoatSessionRhs);
