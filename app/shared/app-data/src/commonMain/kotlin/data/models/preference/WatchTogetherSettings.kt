/*
 * Copyright (C) 2024-2026 OpenAni and contributors.
 *
 * 此源代码的使用受 GNU AFFERO GENERAL PUBLIC LICENSE version 3 许可证的约束, 可以在以下链接找到该许可证.
 * Use of this source code is governed by the GNU AGPLv3 license, which can be found at the following link.
 *
 * https://github.com/open-ani/ani/blob/main/LICENSE
 */

package me.him188.ani.app.data.models.preference

import androidx.compose.runtime.Immutable
import androidx.compose.runtime.Stable
import kotlinx.serialization.Serializable

@Immutable
@Serializable
data class WatchTogetherSettings(
    val enabled: Boolean = false,
    val followHost: Boolean = true,
    val lastRoomName: String = "",
    val rememberedSession: RememberedRoomSession? = null,
    /**
     * 聊天**扩展**服务链接。
     *
     * 一起看的房间、成员与播放同步始终由官方服务端提供, 本字段只用于接入聊天扩展服务。
     * 支持域名或 ip:端口, 例如 `example.com`、`192.168.1.10:8080`、`https://example.com`。
     * 为空表示未启用聊天扩展。
     */
    val chatExtensionUrl: String = "",
) {
    companion object {
        @Stable
        val Default = WatchTogetherSettings()
    }
}

@Immutable
@Serializable
data class RememberedRoomSession(
    val roomName: String,
    val password: String,
    val joinedAt: Long,
)
