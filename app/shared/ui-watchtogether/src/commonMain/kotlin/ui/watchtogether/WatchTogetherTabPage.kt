/*
 * Copyright (C) 2024-2026 OpenAni and contributors.
 *
 * 此源代码的使用受 GNU AFFERO GENERAL PUBLIC LICENSE version 3 许可证的约束, 可以在以下链接找到该许可证.
 * Use of this source code is governed by the GNU AGPLv3 license, which can be found at the following link.
 *
 * https://github.com/open-ani/ani/blob/main/LICENSE
 */

package me.him188.ani.app.ui.watchtogether

import androidx.compose.animation.AnimatedContent
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.lazy.rememberLazyListState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.rounded.Send
import androidx.compose.material.icons.rounded.Settings
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.DropdownMenu
import androidx.compose.material3.DropdownMenuItem
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import me.him188.ani.app.data.network.WatchTogetherChatMessage
import me.him188.ani.app.ui.foundation.avatar.AvatarImage
import me.him188.ani.app.ui.lang.Lang
import me.him188.ani.app.ui.lang.watch_together_cancel
import me.him188.ani.app.ui.lang.watch_together_chat_extension
import me.him188.ani.app.ui.lang.watch_together_chat_extension_disable
import me.him188.ani.app.ui.lang.watch_together_chat_extension_hint
import me.him188.ani.app.ui.lang.watch_together_chat_extension_placeholder
import me.him188.ani.app.ui.lang.watch_together_chat_input_placeholder
import me.him188.ani.app.ui.lang.watch_together_chat_send
import me.him188.ani.app.ui.lang.watch_together_join
import me.him188.ani.app.ui.lang.watch_together_join_helper
import me.him188.ani.app.ui.lang.watch_together_joining
import me.him188.ani.app.ui.lang.watch_together_leave
import me.him188.ani.app.ui.lang.watch_together_password
import me.him188.ani.app.ui.lang.watch_together_room_name
import me.him188.ani.app.ui.lang.watch_together_save
import me.him188.ani.app.ui.lang.watch_together_settings
import org.jetbrains.compose.resources.stringResource

internal const val WATCH_TOGETHER_CHAT_INPUT_TEST_TAG = "watch_together_chat_input"
internal const val WATCH_TOGETHER_CHAT_SEND_TEST_TAG = "watch_together_chat_send"

internal fun watchTogetherChatBubbleTestTag(messageId: String) = "watch_together_chat_bubble_$messageId"
internal fun watchTogetherChatNicknameTestTag(messageId: String) = "watch_together_chat_nickname_$messageId"
internal fun watchTogetherChatAvatarTestTag(messageId: String) = "watch_together_chat_avatar_$messageId"

/**
 * 「一起看」Tab 页面:
 * - 未加入房间时展示加入表单(房间由官方服务端提供);
 * - 已加入房间时展示聊天室(消息列表 + 输入框), 自己的消息靠右、他人的靠左;
 * - 右上角设置可配置聊天**扩展**链接, 房间本身始终来自官方源。
 *
 * @param selfUserId 自己在官方房间里的成员 id, 用于把消息区分成"我"与"他人"。
 * 消息的昵称与头像取自官方房间成员列表, 因此与官方「一起看」保持同步。
 */
@Composable
fun WatchTogetherTabPage(
    state: WatchTogetherUiState,
    messages: List<WatchTogetherChatMessage>,
    extensionUrl: String,
    onJoinRoom: (roomName: String, password: String) -> Unit,
    onLeaveRoom: () -> Unit,
    onSendMessage: (String) -> Unit,
    onExtensionUrlChange: (String) -> Unit,
    selfUserId: String? = null,
    modifier: Modifier = Modifier,
) {
    Column(modifier.fillMaxSize()) {
        // 右上角设置
        Row(
            Modifier.fillMaxWidth().padding(horizontal = 8.dp),
            horizontalArrangement = Arrangement.End,
        ) {
            ChatExtensionMenu(
                extensionUrl = extensionUrl,
                onExtensionUrlChange = onExtensionUrlChange,
            )
        }

        AnimatedContent(
            targetState = state.phase,
            modifier = Modifier.fillMaxWidth().weight(1f),
            label = "WatchTogetherTabPhase",
        ) { phase ->
            when (phase) {
                WatchTogetherPhase.IN_ROOM -> ChatRoom(
                    messages = messages,
                    selfUserId = selfUserId,
                    onSendMessage = onSendMessage,
                    onLeaveRoom = onLeaveRoom,
                    modifier = Modifier.fillMaxSize(),
                )

                else -> JoinRoomPane(
                    state = state,
                    onJoinRoom = onJoinRoom,
                    modifier = Modifier.fillMaxSize(),
                )
            }
        }
    }
}

/**
 * 右上角设置:配置聊天扩展链接。房间本身由官方服务端提供, 此处不影响一起看房间。
 */
@Composable
private fun ChatExtensionMenu(
    extensionUrl: String,
    onExtensionUrlChange: (String) -> Unit,
    modifier: Modifier = Modifier,
) {
    var expanded by remember { mutableStateOf(false) }
    var editing by remember { mutableStateOf(false) }

    Box(modifier) {
        IconButton(onClick = { expanded = true }) {
            Icon(
                Icons.Rounded.Settings,
                contentDescription = stringResource(Lang.watch_together_settings),
            )
        }
        DropdownMenu(expanded = expanded, onDismissRequest = { expanded = false }) {
            DropdownMenuItem(
                text = { Text(stringResource(Lang.watch_together_chat_extension)) },
                onClick = {
                    expanded = false
                    editing = true
                },
            )
            DropdownMenuItem(
                text = { Text(stringResource(Lang.watch_together_chat_extension_disable)) },
                onClick = {
                    expanded = false
                    onExtensionUrlChange("")
                },
            )
        }
    }

    if (editing) {
        ChatExtensionDialog(
            initialValue = extensionUrl,
            onConfirm = {
                onExtensionUrlChange(it)
                editing = false
            },
            onDismiss = { editing = false },
        )
    }
}

@Composable
private fun ChatExtensionDialog(
    initialValue: String,
    onConfirm: (String) -> Unit,
    onDismiss: () -> Unit,
) {
    var text by remember(initialValue) { mutableStateOf(initialValue) }
    AlertDialog(
        onDismissRequest = onDismiss,
        title = { Text(stringResource(Lang.watch_together_chat_extension)) },
        text = {
            Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
                OutlinedTextField(
                    value = text,
                    onValueChange = { text = it },
                    placeholder = {
                        Text(stringResource(Lang.watch_together_chat_extension_placeholder))
                    },
                    singleLine = true,
                    modifier = Modifier.fillMaxWidth(),
                )
                Text(
                    stringResource(Lang.watch_together_chat_extension_hint),
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                )
            }
        },
        confirmButton = {
            TextButton(onClick = { onConfirm(text.trim()) }) {
                Text(stringResource(Lang.watch_together_save))
            }
        },
        dismissButton = {
            TextButton(onClick = onDismiss) {
                Text(stringResource(Lang.watch_together_cancel))
            }
        },
    )
}

/**
 * 加入房间表单。房间名不存在时服务端会自动创建。
 */
@Composable
private fun JoinRoomPane(
    state: WatchTogetherUiState,
    onJoinRoom: (roomName: String, password: String) -> Unit,
    modifier: Modifier = Modifier,
) {
    var roomName by remember(state.joinForm.lastRoomName) {
        mutableStateOf(state.joinForm.lastRoomName)
    }
    var password by remember { mutableStateOf("") }

    Column(
        modifier.padding(16.dp),
        verticalArrangement = Arrangement.spacedBy(12.dp),
    ) {
        OutlinedTextField(
            value = roomName,
            onValueChange = { roomName = it },
            label = { Text(stringResource(Lang.watch_together_room_name)) },
            singleLine = true,
            modifier = Modifier.fillMaxWidth(),
        )
        OutlinedTextField(
            value = password,
            onValueChange = { password = it },
            label = { Text(stringResource(Lang.watch_together_password)) },
            singleLine = true,
            modifier = Modifier.fillMaxWidth(),
        )
        state.joinForm.errorMessage?.let { error ->
            Text(error, color = MaterialTheme.colorScheme.error)
        }
        TextButton(
            onClick = { onJoinRoom(roomName.trim(), password) },
            enabled = roomName.isNotBlank() && state.phase != WatchTogetherPhase.JOINING,
        ) {
            Text(
                if (state.phase == WatchTogetherPhase.JOINING) {
                    stringResource(Lang.watch_together_joining)
                } else {
                    stringResource(Lang.watch_together_join)
                },
            )
        }
        Text(
            stringResource(Lang.watch_together_join_helper),
            style = MaterialTheme.typography.bodySmall,
        )
    }
}

/**
 * 聊天室:消息列表 + 底部输入框。
 *
 * 自己的消息靠右、他人的靠左, 两侧都显示官方房间成员的头像与昵称。
 */
@Composable
private fun ChatRoom(
    messages: List<WatchTogetherChatMessage>,
    selfUserId: String?,
    onSendMessage: (String) -> Unit,
    onLeaveRoom: () -> Unit,
    modifier: Modifier = Modifier,
) {
    var input by remember { mutableStateOf("") }
    val listState = rememberLazyListState()

    LaunchedEffect(messages.size) {
        if (messages.isNotEmpty()) {
            listState.animateScrollToItem(messages.lastIndex)
        }
    }

    Column(modifier) {
        LazyColumn(
            Modifier.fillMaxWidth().weight(1f).padding(horizontal = 12.dp),
            state = listState,
            verticalArrangement = Arrangement.spacedBy(10.dp),
        ) {
            items(messages, key = { it.id }) { message ->
                ChatMessageRow(
                    message = message,
                    isSelf = selfUserId != null && message.userId == selfUserId,
                )
            }
        }

        Row(
            Modifier.fillMaxWidth().padding(8.dp),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            OutlinedTextField(
                value = input,
                onValueChange = { input = it },
                placeholder = {
                    Text(stringResource(Lang.watch_together_chat_input_placeholder))
                },
                modifier = Modifier.weight(1f).heightIn(min = 48.dp).testTag(WATCH_TOGETHER_CHAT_INPUT_TEST_TAG),
                singleLine = true,
            )
            IconButton(
                onClick = {
                    val text = input.trim()
                    if (text.isNotEmpty()) {
                        onSendMessage(text)
                        input = ""
                    }
                },
                enabled = input.isNotBlank(),
                modifier = Modifier.testTag(WATCH_TOGETHER_CHAT_SEND_TEST_TAG),
            ) {
                Icon(
                    Icons.AutoMirrored.Rounded.Send,
                    contentDescription = stringResource(Lang.watch_together_chat_send),
                )
            }
        }

        TextButton(onClick = onLeaveRoom, modifier = Modifier.padding(horizontal = 8.dp)) {
            Text(stringResource(Lang.watch_together_leave))
        }
    }
}

/**
 * 一条聊天消息。系统消息居中; 其余按 [isSelf] 分到右侧(自己)或左侧(他人)。
 *
 * 头像与昵称都来自官方房间成员列表, 与官方「一起看」面板一致。
 */
@Composable
private fun ChatMessageRow(
    message: WatchTogetherChatMessage,
    isSelf: Boolean,
    modifier: Modifier = Modifier,
) {
    if (message.system) {
        Text(
            message.content,
            style = MaterialTheme.typography.bodySmall,
            color = MaterialTheme.colorScheme.onSurfaceVariant,
            textAlign = TextAlign.Center,
            modifier = modifier.padding(vertical = 2.dp).fillMaxWidth(),
        )
        return
    }

    Row(
        modifier = modifier.fillMaxWidth(),
        horizontalArrangement = if (isSelf) Arrangement.End else Arrangement.Start,
        verticalAlignment = Alignment.Top,
    ) {
        if (!isSelf) {
            ChatAvatar(message, Modifier.padding(end = 8.dp))
        }
        Column(
            horizontalAlignment = if (isSelf) Alignment.End else Alignment.Start,
            modifier = Modifier.widthIn(max = 260.dp),
        ) {
            Text(
                message.nickname,
                style = MaterialTheme.typography.labelMedium,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
                maxLines = 1,
                overflow = TextOverflow.Ellipsis,
                modifier = Modifier.testTag(watchTogetherChatNicknameTestTag(message.id)),
            )
            Surface(
                color = if (isSelf) {
                    MaterialTheme.colorScheme.primaryContainer
                } else {
                    MaterialTheme.colorScheme.surfaceContainerHigh
                },
                contentColor = if (isSelf) {
                    MaterialTheme.colorScheme.onPrimaryContainer
                } else {
                    MaterialTheme.colorScheme.onSurface
                },
                shape = chatBubbleShape(isSelf),
                modifier = Modifier
                    .padding(top = 2.dp)
                    .testTag(watchTogetherChatBubbleTestTag(message.id)),
            ) {
                Text(
                    message.content,
                    style = MaterialTheme.typography.bodyMedium,
                    modifier = Modifier.padding(horizontal = 12.dp, vertical = 8.dp),
                )
            }
        }
        if (isSelf) {
            ChatAvatar(message, Modifier.padding(start = 8.dp))
        }
    }
}

/** 消息头像: 与官方房间成员列表使用同一张图, 缺失时回退到占位头像。 */
@Composable
private fun ChatAvatar(message: WatchTogetherChatMessage, modifier: Modifier = Modifier) {
    AvatarImage(
        message.avatarUrl,
        modifier = modifier
            .size(34.dp)
            .clip(CircleShape)
            .testTag(watchTogetherChatAvatarTestTag(message.id)),
    )
}

/** 气泡靠向自己一侧的那两个角收窄, 让左右分栏一眼可辨。 */
private fun chatBubbleShape(isSelf: Boolean) = RoundedCornerShape(
    topStart = 12.dp,
    topEnd = 12.dp,
    bottomStart = if (isSelf) 12.dp else 2.dp,
    bottomEnd = if (isSelf) 2.dp else 12.dp,
)