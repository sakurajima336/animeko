/*
 * Copyright (C) 2024-2026 OpenAni and contributors.
 *
 * 此源代码的使用受 GNU AFFERO GENERAL PUBLIC LICENSE version 3 许可证的约束, 可以在以下链接找到该许可证.
 * Use of this source code is governed by the GNU AGPLv3 license, which can be found at the following link.
 *
 * https://github.com/open-ani/ani/blob/main/LICENSE
 */

package me.him188.ani.app.data.network

import io.ktor.client.call.body
import io.ktor.client.request.get
import io.ktor.client.request.post
import io.ktor.client.request.setBody
import io.ktor.http.ContentType
import io.ktor.http.URLBuilder
import io.ktor.http.appendPathSegments
import io.ktor.http.contentType
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.first
import kotlinx.serialization.json.Json
import me.him188.ani.utils.ktor.ScopedHttpClient
import me.him188.ani.utils.ktor.UnsafeScopedHttpClientApi

/**
 * 官方一起看房间的**聊天扩展**。
 *
 * 房间本身由官方服务端提供(join / report / leave / events), 扩展只是给该房间附加一个聊天室。
 * 扩展按官方下发的 `roomId` 定位房间, 因此聊天室与官方房间**严格绑定**:
 * 房间关闭或更换后, 旧 roomId 不再有消息。
 *
 * 未配置扩展链接时, 所有调用都返回空结果, 不影响官方一起看的正常使用。
 */
class DefaultWatchTogetherChatApi(
    private val client: ScopedHttpClient,
    /** 聊天扩展服务链接。为空表示未启用扩展。 */
    private val extensionUrl: Flow<String>,
    private val json: Json = Json { ignoreUnknownKeys = true },
) : WatchTogetherChatApi {
    /**
     * 把用户填写的链接规范化为绝对 URL。
     * 未写 scheme 时默认 http, 局域网自建服务通常没有证书。
     */
    private fun baseUrl(raw: String): String? {
        val trimmed = raw.trim().trimEnd('/')
        if (trimmed.isEmpty()) return null
        val withScheme = if (trimmed.contains("://")) trimmed else "http://$trimmed"
        return if (withScheme.endsWith("/")) withScheme else "$withScheme/"
    }

    @OptIn(UnsafeScopedHttpClientApi::class)
    override suspend fun fetchHistory(roomId: String): List<WatchTogetherChatMessage> {
        // roomId 来自官方服务端, 保证聊天室与官方房间一一对应。
        val base = baseUrl(extensionUrl.first()) ?: return emptyList()
        val url = URLBuilder(base).apply {
            appendPathSegments("v2", "watch-together", "rooms", roomId, "chat")
        }.buildString()
        return client.use {
            runCatching {
                get(url).body<WatchTogetherChatHistoryResponse>().messages
            }.getOrDefault(emptyList())
        }
    }

    @OptIn(UnsafeScopedHttpClientApi::class)
    override suspend fun send(
        roomId: String,
        sessionNonce: String,
        content: String,
    ): WatchTogetherChatMessage? {
        val base = baseUrl(extensionUrl.first()) ?: return null
        val url = URLBuilder(base).apply {
            appendPathSegments("v2", "watch-together", "rooms", roomId, "chat")
        }.buildString()
        return client.use {
            runCatching {
                post(url) {
                    contentType(ContentType.Application.Json)
                    setBody(WatchTogetherSendChatRequest(sessionNonce, content))
                }.body<WatchTogetherChatMessage>()
            }.getOrNull()
        }
    }
}