/*
 * Copyright (C) 2024-2026 OpenAni and contributors.
 *
 * 此源代码的使用受 GNU AFFERO GENERAL PUBLIC LICENSE version 3 许可证的约束, 可以在以下链接找到该许可证.
 * Use of this source code is governed by the GNU AGPLv3 license, which can be found at the following link.
 *
 * https://github.com/open-ani/ani/blob/main/LICENSE
 */

package me.him188.ani.app.data.network

import kotlinx.coroutines.flow.Flow
import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable
import kotlinx.serialization.Transient

/**
 * 房间聊天消息。由自托管服务端提供,官方协议未定义该结构。
 *
 * [userId] / [nickname] / [avatarUrl] 来自官方一起看房间的成员信息(由发言客户端透传),
 * 客户端据此把消息归属到官方成员, 使聊天室里的昵称与头像与官方房间一致。
 */
@Serializable
data class WatchTogetherChatMessage(
    @SerialName("id") val id: String,
    @SerialName("roomId") val roomId: String,
    @SerialName("userId") val userId: String,
    @SerialName("nickname") val nickname: String,
    @SerialName("avatarUrl") val avatarUrl: String? = null,
    @SerialName("content") val content: String,
    @SerialName("sentAt") val sentAt: Long,
    @SerialName("system") val system: Boolean = false,
    /**
     * 客户端生成的幂等标识, 服务端会原样回传。
     *
     * 用于"乐观上屏": 点发送时本地立刻插入一条带该 id 的消息, 待 SSE 推回同 id 的服务端消息后
     * 自动去重, 从而无需等待服务端往返即可看到自己的消息。
     */
    @SerialName("clientMessageId") val clientMessageId: String? = null,
    /** 仅本地使用: 该消息是否仍在发送中(尚未收到服务端回执)。 */
    @Transient val pending: Boolean = false,
) {
    /** 去重键: 优先用客户端幂等 id, 使乐观消息与服务端消息能相互抵消。 */
    val dedupKey: String get() = clientMessageId ?: id
}

/**
 * 发言者的官方成员信息, 随发送请求透传给聊天扩展。
 */
data class WatchTogetherChatSender(
    val userId: String,
    val nickname: String,
    val avatarUrl: String? = null,
)

@Serializable
data class WatchTogetherSendChatRequest(
    @SerialName("sessionNonce") val sessionNonce: String,
    @SerialName("content") val content: String,
    @SerialName("senderUserId") val senderUserId: String? = null,
    @SerialName("senderNickname") val senderNickname: String? = null,
    @SerialName("senderAvatarUrl") val senderAvatarUrl: String? = null,
    /** 客户端幂等 id, 服务端原样回传, 供乐观上屏去重。 */
    @SerialName("clientMessageId") val clientMessageId: String? = null,
)

@Serializable
data class WatchTogetherChatHistoryResponse(
    @SerialName("serverTime") val serverTime: Long,
    @SerialName("messages") val messages: List<WatchTogetherChatMessage>,
)

/**
 * 聊天扩展的服务端事件。
 */
sealed interface WatchTogetherChatEvent {
    data object Connected : WatchTogetherChatEvent
    data object Ping : WatchTogetherChatEvent
    data class Chat(val message: WatchTogetherChatMessage) : WatchTogetherChatEvent
}

/**
 * SSE `event: chat` 的 data 载荷。
 *
 * 服务端下发的 data 形如 `{"message":{...}}`, 消息本体在外层 `message` 字段里,
 * 不能直接按 [WatchTogetherChatMessage] 解析。
 */
@Serializable
internal data class WatchTogetherChatEventPayload(
    @SerialName("message") val message: WatchTogetherChatMessage,
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
     *
     * [sender] 是发言者在官方房间里的成员信息, 服务端据此让聊天室显示与官方房间一致。
     * [clientMessageId] 为客户端幂等 id, 服务端会原样回传, 供乐观上屏去重。
     */
    suspend fun send(
        roomId: String,
        sessionNonce: String,
        content: String,
        sender: WatchTogetherChatSender?,
        clientMessageId: String? = null,
    ): WatchTogetherChatMessage?

    /**
     * 监听房间聊天事件流 (SSE)。
     * 实时接收新消息,避免轮询延迟。
     */
    fun events(roomId: String): Flow<WatchTogetherChatEvent>
}