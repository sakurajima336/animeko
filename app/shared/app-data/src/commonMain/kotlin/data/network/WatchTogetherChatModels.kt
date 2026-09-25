/*
 * Copyright (C) 2024-2026 OpenAni and contributors.
 *
 * 此源代码的使用受 GNU AFFERO GENERAL PUBLIC LICENSE version 3 许可证的约束, 可以在以下链接找到该许可证.
 * Use of this source code is governed by the GNU AGPLv3 license, which can be found at the following link.
 *
 * https://github.com/open-ani/ani/blob/main/LICENSE
 */

package me.him188.ani.app.data.network

import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable

/**
 * 房间聊天消息。由自托管服务端提供,官方协议未定义该结构。
 */
@Serializable
data class WatchTogetherChatMessage(
    @SerialName("id") val id: String,
    @SerialName("roomId") val roomId: String,
    @SerialName("userId") val userId: String,
    @SerialName("nickname") val nickname: String,
    @SerialName("content") val content: String,
    @SerialName("sentAt") val sentAt: Long,
    @SerialName("system") val system: Boolean = false,
)

@Serializable
data class WatchTogetherSendChatRequest(
    @SerialName("sessionNonce") val sessionNonce: String,
    @SerialName("content") val content: String,
)

@Serializable
data class WatchTogetherChatHistoryResponse(
    @SerialName("serverTime") val serverTime: Long,
    @SerialName("messages") val messages: List<WatchTogetherChatMessage>,
)

/**
 * 房间聊天能力。房间之间消息隔离由服务端保证。
 */
interface WatchTogetherChatApi {
    /**
     * 拉取房间聊天历史。
     */
    suspend fun fetchHistory(roomId: String): List<WatchTogetherChatMessage>

    /**
     * 发送一条消息。返回服务端落库后的消息,失败时返回 null。
     */
    suspend fun send(roomId: String, sessionNonce: String, content: String): WatchTogetherChatMessage?
}