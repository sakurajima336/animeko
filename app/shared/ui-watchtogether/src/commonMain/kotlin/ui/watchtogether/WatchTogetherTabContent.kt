/*
 * Copyright (C) 2024-2026 OpenAni and contributors.
 *
 * 此源代码的使用受 GNU AFFERO GENERAL PUBLIC LICENSE version 3 许可证的约束, 可以在以下链接找到该许可证.
 * Use of this source code is governed by the GNU AGPLv3 license, which can be found at the following link.
 *
 * https://github.com/open-ani/ani/blob/main/LICENSE
 */

package me.him188.ani.app.ui.watchtogether

import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.ui.Modifier
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import androidx.lifecycle.viewmodel.compose.viewModel

/**
 * 播放页竖屏「一起看」Tab 的内容。
 *
 * 自带 [WatchTogetherViewModel],因此宿主页面(播放页)无需感知一起看的状态。
 */
@Composable
fun WatchTogetherTabContent(
    modifier: Modifier = Modifier,
    viewModel: WatchTogetherViewModel = viewModel { WatchTogetherViewModel() },
) {
    val uiState by viewModel.uiStateFlow.collectAsStateWithLifecycle()
    val messages by viewModel.chatMessages.collectAsStateWithLifecycle()
    val extensionUrl by viewModel.extensionUrl.collectAsStateWithLifecycle()
    val selfUserId by viewModel.selfUserId.collectAsStateWithLifecycle()

    WatchTogetherTabPage(
        state = uiState,
        messages = messages,
        extensionUrl = extensionUrl,
        onJoinRoom = { roomName, password ->
            viewModel.onIntent(WatchTogetherIntent.JoinRoom(roomName, password))
        },
        onLeaveRoom = {
            viewModel.onIntent(WatchTogetherIntent.LeaveRoom)
        },
        onSendMessage = { viewModel.sendChatMessage(it) },
        onExtensionUrlChange = { viewModel.setExtensionUrl(it) },
        selfUserId = selfUserId,
        modifier = modifier,
    )
}