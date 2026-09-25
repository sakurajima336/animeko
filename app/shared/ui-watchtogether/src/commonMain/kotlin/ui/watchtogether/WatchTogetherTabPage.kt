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
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.lazy.rememberLazyListState
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
import androidx.compose.ui.unit.dp
import me.him188.ani.app.data.network.WatchTogetherChatMessage
import me.him188.ani.app.ui.lang.Lang
import me.him188.ani.app.ui.lang.watch_together_cancel
import me.him188.ani.app.ui.lang.watch_together_chat_input_placeholder
import me.him188.ani.app.ui.lang.watch_together_chat_send
import me.him188.ani.app.ui.lang.watch_together_join
import me.him188.ani.app.ui.lang.watch_together_join_helper
import me.him188.ani.app.ui.lang.watch_together_joining
import me.him188.ani.app.ui.lang.watch_together_leave
import me.him188.ani.app.ui.lang.watch_together_password
import me.him188.ani.app.ui.lang.watch_together_room_name
import me.him188.ani.app.ui.lang.watch_together_save
import me.him188.ani.app.ui.lang.watch_together_server_address
import me.him188.ani.app.ui.lang.watch_together_server_address_placeholder
import me.him188.ani.app.ui.lang.watch_together_settings
import me.him188.ani.app.ui.lang.watch_together_use_official_server
import org.jetbrains.compose.resources.stringResource

/**
 * 「一起看」Tab 页面:
 * - 未加入房间时展示加入表单;
 * - 已加入房间时展示聊天室(消息列表 + 输入框);
 * - 右上角设置菜单可配置自托管服务端地址(域名或 ip:端口)。
 */
@Composable
fun WatchTogetherTabPage(
    state: WatchTogetherUiState,
    messages: List<WatchTogetherChatMessage>,
    serverAddress: String,
    onJoinRoom: (roomName: String, password: String) -> Unit,
    onLeaveRoom: () -> Unit,
    onSendMessage: (String) -> Unit,
    onServerAddressChange: (String) -> Unit,
    modifier: Modifier = Modifier,
) {
    Column(modifier.fillMaxSize()) {
        // 右上角设置
        Row(
            Modifier.fillMaxWidth().padding(horizontal = 8.dp),
            horizontalArrangement = Arrangement.End,
        ) {
            ServerAddressMenu(
                serverAddress = serverAddress,
                onServerAddressChange = onServerAddressChange,
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
 * 右上角设置菜单:展开后可设置一起看的服务端地址。
 */
@Composable
private fun ServerAddressMenu(
    serverAddress: String,
    onServerAddressChange: (String) -> Unit,
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
                text = { Text(stringResource(Lang.watch_together_server_address)) },
                onClick = {
                    expanded = false
                    editing = true
                },
            )
            DropdownMenuItem(
                text = { Text(stringResource(Lang.watch_together_use_official_server)) },
                onClick = {
                    expanded = false
                    onServerAddressChange("")
                },
            )
        }
    }

    if (editing) {
        ServerAddressDialog(
            initialValue = serverAddress,
            onConfirm = {
                onServerAddressChange(it)
                editing = false
            },
            onDismiss = { editing = false },
        )
    }
}

@Composable
private fun ServerAddressDialog(
    initialValue: String,
    onConfirm: (String) -> Unit,
    onDismiss: () -> Unit,
) {
    var text by remember(initialValue) { mutableStateOf(initialValue) }
    AlertDialog(
        onDismissRequest = onDismiss,
        title = { Text(stringResource(Lang.watch_together_server_address)) },
        text = {
            OutlinedTextField(
                value = text,
                onValueChange = { text = it },
                placeholder = {
                    Text(stringResource(Lang.watch_together_server_address_placeholder))
                },
                singleLine = true,
                modifier = Modifier.fillMaxWidth(),
            )
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
 */
@Composable
private fun ChatRoom(
    messages: List<WatchTogetherChatMessage>,
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
            verticalArrangement = Arrangement.spacedBy(6.dp),
        ) {
            items(messages, key = { it.id }) { message ->
                ChatBubble(message)
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
                modifier = Modifier.weight(1f).heightIn(min = 48.dp),
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

@Composable
private fun ChatBubble(message: WatchTogetherChatMessage) {
    if (message.system) {
        Text(
            message.content,
            style = MaterialTheme.typography.bodySmall,
            color = MaterialTheme.colorScheme.onSurfaceVariant,
            modifier = Modifier.padding(vertical = 2.dp).fillMaxWidth(),
        )
        return
    }
    Surface(
        color = MaterialTheme.colorScheme.surfaceContainerHigh,
        shape = RoundedCornerShape(12.dp),
    ) {
        Column(Modifier.padding(horizontal = 12.dp, vertical = 6.dp)) {
            Text(
                message.nickname,
                style = MaterialTheme.typography.labelMedium,
                color = MaterialTheme.colorScheme.primary,
            )
            Text(message.content, style = MaterialTheme.typography.bodyMedium)
        }
    }
}