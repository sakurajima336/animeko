/*
 * Copyright (C) 2024-2026 OpenAni and contributors.
 *
 * 此源代码的使用受 GNU AFFERO GENERAL PUBLIC LICENSE version 3 许可证的约束, 可以在以下链接找到该许可证.
 * Use of this source code is governed by the GNU AGPLv3 license, which can be found at the following link.
 *
 * https://github.com/open-ani/ani/blob/main/LICENSE
 */

package me.him188.ani.app.ui.watchtogether

import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.size
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.test.assertIsDisplayed
import androidx.compose.ui.test.getBoundsInRoot
import androidx.compose.ui.test.onNodeWithTag
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.performTextInput
import androidx.compose.ui.unit.dp
import me.him188.ani.app.data.network.WatchTogetherChatMessage
import me.him188.ani.app.ui.foundation.ProvideCompositionLocalsForPreview
import me.him188.ani.app.ui.framework.runAniComposeUiTest
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

/**
 * 「一起看」Tab 聊天室: 消息左右分栏、昵称与头像取自官方房间成员。
 */
class WatchTogetherChatTabTest {
    @Test
    fun `own messages align right and others align left`() = runAniComposeUiTest {
        setContent {
            ProvideCompositionLocalsForPreview {
                Box(Modifier.size(360.dp, 640.dp).testTag(CONTAINER_TEST_TAG)) {
                    WatchTogetherTabPage(
                        state = inRoomState(),
                        messages = listOf(
                            chatMessage(id = "m1", userId = "u-me", nickname = "我", content = "在的"),
                            chatMessage(id = "m2", userId = "u-other", nickname = "对方", content = "你好"),
                        ),
                        extensionUrl = "",
                        onJoinRoom = { _, _ -> },
                        onLeaveRoom = {},
                        onSendMessage = {},
                        onExtensionUrlChange = {},
                        selfUserId = "u-me",
                    )
                }
            }
        }

        val container = onNodeWithTag(CONTAINER_TEST_TAG).getBoundsInRoot()
        val mine = onNodeWithTag(watchTogetherChatBubbleTestTag("m1")).getBoundsInRoot()
        val theirs = onNodeWithTag(watchTogetherChatBubbleTestTag("m2")).getBoundsInRoot()

        // 自己的消息贴右: 距右边缘比对方的近得多。
        assertTrue(
            container.right - mine.right < container.right - theirs.right,
            "own bubble $mine should sit closer to the right edge of $container than other's $theirs",
        )
        // 他人的消息贴左: 距左边缘比自己的近得多。
        assertTrue(
            theirs.left - container.left < mine.left - container.left,
            "other's bubble $theirs should sit closer to the left edge of $container than own $mine",
        )
        onNodeWithText("对方").assertIsDisplayed()
    }

    @Test
    fun `each message shows the official avatar on its own side`() = runAniComposeUiTest {
        setContent {
            ProvideCompositionLocalsForPreview {
                Box(Modifier.size(360.dp, 640.dp).testTag(CONTAINER_TEST_TAG)) {
                    WatchTogetherTabPage(
                        state = inRoomState(),
                        messages = listOf(
                            chatMessage(id = "m1", userId = "u-me", nickname = "我", content = "在的"),
                            chatMessage(id = "m2", userId = "u-other", nickname = "对方", content = "你好"),
                        ),
                        extensionUrl = "",
                        onJoinRoom = { _, _ -> },
                        onLeaveRoom = {},
                        onSendMessage = {},
                        onExtensionUrlChange = {},
                        selfUserId = "u-me",
                    )
                }
            }
        }

        val mine = onNodeWithTag(watchTogetherChatBubbleTestTag("m1")).getBoundsInRoot()
        val myAvatar = onNodeWithTag(watchTogetherChatAvatarTestTag("m1")).getBoundsInRoot()
        val theirBubble = onNodeWithTag(watchTogetherChatBubbleTestTag("m2")).getBoundsInRoot()
        val theirAvatar = onNodeWithTag(watchTogetherChatAvatarTestTag("m2")).getBoundsInRoot()

        // 自己的头像在气泡右侧, 对方的在气泡左侧。
        assertTrue(myAvatar.left >= mine.right, "own avatar $myAvatar should sit right of own bubble $mine")
        assertTrue(
            theirAvatar.right <= theirBubble.left,
            "other's avatar $theirAvatar should sit left of their bubble $theirBubble",
        )
    }

    @Test
    fun `sending a message dispatches the typed text`() = runAniComposeUiTest {
        var sent: String? = null
        setContent {
            ProvideCompositionLocalsForPreview {
                Box(Modifier.size(360.dp, 640.dp)) {
                    WatchTogetherTabPage(
                        state = inRoomState(),
                        messages = emptyList(),
                        extensionUrl = "",
                        onJoinRoom = { _, _ -> },
                        onLeaveRoom = {},
                        onSendMessage = { sent = it },
                        onExtensionUrlChange = {},
                        selfUserId = "u-me",
                    )
                }
            }
        }

        onNodeWithTag(WATCH_TOGETHER_CHAT_INPUT_TEST_TAG).performTextInput(" 一起看真好 ")
        onNodeWithTag(WATCH_TOGETHER_CHAT_SEND_TEST_TAG).performClick()
        runOnIdle { assertEquals("一起看真好", sent) }
    }

    private fun chatMessage(id: String, userId: String, nickname: String, content: String) =
        WatchTogetherChatMessage(
            id = id,
            roomId = "r_official",
            userId = userId,
            nickname = nickname,
            avatarUrl = null,
            content = content,
            sentAt = 1_790_352_808_861L,
        )

    private fun inRoomState() = WatchTogetherUiState(
        featureEnabled = true,
        phase = WatchTogetherPhase.IN_ROOM,
        joinForm = WatchTogetherJoinFormState(lastRoomName = "周五夜放送"),
        room = WatchTogetherRoomCardState(
            roomName = "周五夜放送",
            connection = WatchTogetherConnectionPresentation.CONNECTED,
            playback = null,
            members = emptyList(),
        ),
        following = true,
        isSelfHost = false,
        requiresLogin = false,
        inPlayer = false,
    )

    private companion object {
        const val CONTAINER_TEST_TAG = "watch_together_chat_test_container"
    }
}