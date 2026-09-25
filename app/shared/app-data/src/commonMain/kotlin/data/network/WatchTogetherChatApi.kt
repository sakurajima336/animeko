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
import me.him188.ani.app.domain.foundation.ServerListFeatureConfig
import me.him188.ani.utils.ktor.ScopedHttpClient
import me.him188.ani.utils.ktor.UnsafeScopedHttpClientApi
import me.him188.ani.utils.ktor.use

/**
 * 房间聊天的默认实现。与 [WatchTogetherApiService] 共用同一个服务端地址,
 * 因此聊天消息与房间状态始终落在同一台服务器上,保证房间隔离。
 */
class DefaultWatchTogetherChatApi(
    private val client: ScopedHttpClient,
    private val serverBaseUrl: Flow<String>,
    private val json: Json = Json { ignoreUnknownKeys = true },
) : WatchTogetherChatApi {
    private fun baseUrl(raw: String): String {
        val trimmed = raw.trim().trimEnd('/')
        if (trimmed.isEmpty()) return ServerListFeatureConfig.MAGIC_ANI_SERVER
        val withScheme = if (trimmed.contains("://")) trimmed else "http://$trimmed"
        return if (withScheme.endsWith("/")) withScheme else "$withScheme/"
    }

    @OptIn(UnsafeScopedHttpClientApi::class)
    override suspend fun fetchHistory(roomId: String): List<WatchTogetherChatMessage> {
        val url = URLBuilder(baseUrl(serverBaseUrl.first())).apply {
            appendPathSegments("v2", "watch-together", "rooms", roomId, "chat")
        }.buildString()
        return client.use {
            runCatching {
                it.get(url).body<WatchTogetherChatHistoryResponse>().messages
            }.getOrDefault(emptyList())
        }
    }

    @OptIn(UnsafeScopedHttpClientApi::class)
    override suspend fun send(
        roomId: String,
        sessionNonce: String,
        content: String,
    ): WatchTogetherChatMessage? {
        val url = URLBuilder(baseUrl(serverBaseUrl.first())).apply {
            appendPathSegments("v2", "watch-together", "rooms", roomId, "chat")
        }.buildString()
        return client.use {
            runCatching {
                it.post(url) {
                    contentType(ContentType.Application.Json)
                    setBody(WatchTogetherSendChatRequest(sessionNonce, content))
                }.body<WatchTogetherChatMessage>()
            }.getOrNull()
        }
    }
}