/*
 * Copyright (C) 2024-2026 OpenAni and contributors.
 *
 * 此源代码的使用受 GNU AFFERO GENERAL PUBLIC LICENSE version 3 许可证的约束, 可以在以下链接找到该许可证.
 * Use of this source code is governed by the GNU AGPLv3 license, which can be found at the following link.
 *
 * https://github.com/open-ani/ani/blob/main/LICENSE
 */

package me.him188.ani.app.ui.watchtogether

import kotlin.random.Random
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.channels.Channel
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.channelFlow
import kotlinx.coroutines.flow.combine
import kotlinx.coroutines.flow.distinctUntilChanged
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.flow.flatMapLatest
import kotlinx.coroutines.flow.flow
import kotlinx.coroutines.flow.flowOf
import kotlinx.coroutines.flow.map
import kotlinx.coroutines.flow.receiveAsFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.isActive
import kotlinx.coroutines.launch
import kotlinx.coroutines.sync.Mutex
import kotlinx.coroutines.sync.withLock
import me.him188.ani.app.data.network.WatchTogetherChatApi
import me.him188.ani.app.data.network.WatchTogetherChatEvent
import me.him188.ani.app.data.network.WatchTogetherChatMessage
import me.him188.ani.app.data.network.WatchTogetherChatSender
import me.him188.ani.app.data.network.WatchTogetherJoinException
import me.him188.ani.app.data.network.WatchTogetherJoinFailure
import me.him188.ani.app.data.repository.user.SettingsRepository
import me.him188.ani.app.domain.session.SessionState
import me.him188.ani.app.domain.session.SessionStateProvider
import me.him188.ani.app.domain.watchtogether.LocalPlaybackBridge
import me.him188.ani.app.domain.watchtogether.RoomSession
import me.him188.ani.app.domain.watchtogether.WatchTogetherConnectionState
import me.him188.ani.app.domain.watchtogether.WatchTogetherEffect
import me.him188.ani.app.domain.watchtogether.WatchTogetherManager
import me.him188.ani.app.domain.watchtogether.WatchTogetherState
import me.him188.ani.app.domain.watchtogether.currentRoomCredentials
import me.him188.ani.app.ui.foundation.AbstractViewModel
import me.him188.ani.app.ui.foundation.launchInBackground
import me.him188.ani.app.ui.user.SelfInfoStateProducer
import org.koin.core.component.KoinComponent
import org.koin.core.component.inject

open class WatchTogetherViewModel : AbstractViewModel(), KoinComponent {
    private val manager: WatchTogetherManager by inject()
    private val settingsRepository: SettingsRepository by inject()
    private val sessionStateProvider: SessionStateProvider by inject()
    private val playbackBridge: LocalPlaybackBridge by inject()
    private val chatApi: WatchTogetherChatApi by inject()
    private val selfInfoProducer = SelfInfoStateProducer(koin = getKoin())

    private val joinError = MutableStateFlow<String?>(null)
    private val dialogOpenRequestChannel = Channel<Unit>(Channel.BUFFERED)

    internal val dialogOpenRequests: Flow<Unit> = dialogOpenRequestChannel.receiveAsFlow()

    /** 官方一起看房间里的成员身份, 用于把聊天消息归属到官方成员(昵称与头像以官方为准)。 */
    private val roomMemberIdentities: StateFlow<List<WatchTogetherMemberIdentity>> = manager.state
        .flatMapLatest { state ->
            when (state) {
                is WatchTogetherState.InRoom -> state.session.snapshot.map { snapshot ->
                    snapshot.members.map { member ->
                        WatchTogetherMemberIdentity(
                            userId = member.userId,
                            nickname = member.nickname,
                            avatarUrl = member.avatarUrl,
                        )
                    }
                }

                else -> flowOf(emptyList<WatchTogetherMemberIdentity>())
            }
        }
        .stateInBackground(emptyList())

    /** 自己的成员标识, 与官方房间一致。用于把聊天消息区分成"我"与"他人"。 */
    val selfUserId: StateFlow<String?> = selfInfoProducer.flow
        .map { it.selfInfo?.id?.toString() }
        .stateInBackground<String?>(initialValue = null)

    private val roomProjection = manager.state.flatMapLatest { state ->
        when (state) {
            WatchTogetherState.Disabled,
            WatchTogetherState.Idle,
            -> flowOf(RoomProjection())

            is WatchTogetherState.Joining -> flowOf(
                RoomProjection(phase = WatchTogetherPhase.JOINING),
            )

            is WatchTogetherState.InRoom -> state.session.presentationFlow()
        }
    }

    val uiStateFlow = combine(
        settingsRepository.watchTogetherSettings.flow,
        sessionStateProvider.stateFlow,
        roomProjection,
        playbackBridge.localWatching,
        joinError,
    ) { settings, sessionState, projection, localWatching, error ->
        WatchTogetherUiState(
            featureEnabled = settings.enabled,
            phase = projection.phase,
            joinForm = WatchTogetherJoinFormState(
                lastRoomName = settings.lastRoomName,
                errorMessage = error,
            ),
            room = projection.room,
            following = projection.following,
            isSelfHost = projection.isSelfHost,
            requiresLogin = settings.enabled && sessionState !is SessionState.Valid,
            inPlayer = localWatching != null,
        )
    }.stateInBackground(WatchTogetherUiState.Initial)

    val effects: Flow<WatchTogetherEffect> = manager.effects

    /**
     * 当前房间的聊天消息, 已按官方房间成员信息补齐昵称/头像。
     *
     * 房间之间消息隔离由服务端保证, 离开房间后清空,避免把上一个房间的消息带到下一个房间。
     * 消息来源优先级: 乐观上屏(本地立即显示) > SSE 实时推送 > 轮询兜底。
     */
    val chatMessages: StateFlow<List<WatchTogetherChatMessage>> = manager.state
        .map { (it as? WatchTogetherState.InRoom)?.session?.roomId }
        .distinctUntilChanged()
        .flatMapLatest { roomId ->
            if (roomId == null) {
                flowOf(emptyList<WatchTogetherChatMessage>())
            } else {
                combine(serverMessageFlow(roomId), pendingOutgoing) { server, pending ->
                    mergeWithPending(roomId, server, pending)
                }
            }
        }
        // 成员列表变化时只重新投影昵称/头像, 不打断消息流。
        .combine(roomMemberIdentities) { messages, members ->
            messages.map { it.withOfficialIdentity(members) }
        }
        .stateInBackground(emptyList())

    /**
     * 乐观上屏的本地消息队列(尚未收到服务端回执)。
     *
     * 点发送时立即插入这里, UI 无需等待网络往返即可显示自己的消息;
     * 收到服务端同 clientMessageId 的消息后移除, 由服务端消息接管。
     */
    private val pendingOutgoing = MutableStateFlow<List<WatchTogetherChatMessage>>(emptyList())

    /**
     * 服务端消息流。
     *
     * SSE 是实时通道, 轮询是**并行**的安全网: 两者都写入同一份本地消息表,
     * 因此即使 SSE 静默断开(网络中断、中间代理超时)也不会漏消息, 最坏延迟等于轮询间隔。
     * 每次 SSE 重连都会重新拉取历史, 补齐断线期间的消息。
     */
    private fun serverMessageFlow(roomId: String): Flow<List<WatchTogetherChatMessage>> = channelFlow {
        // 以 dedupKey 为键的本地消息表, 保证乐观消息与服务端消息、SSE 与轮询之间都不重复。
        val store = LinkedHashMap<String, WatchTogetherChatMessage>()
        val lock = Mutex()

        suspend fun merge(incoming: List<WatchTogetherChatMessage>) {
            if (incoming.isEmpty()) return
            val snapshot = lock.withLock {
                for (message in incoming) {
                    store[message.dedupKey] = message
                }
                while (store.size > MAX_CHAT_HISTORY) {
                    val oldest = store.keys.firstOrNull() ?: break
                    store.remove(oldest)
                }
                store.values.toList()
            }
            // 下游 StateFlow 会按 equals 去重, 这里无需额外判断内容是否变化。
            send(snapshot)
        }

        // 首屏历史。
        merge(runCatching { chatApi.fetchHistory(roomId) }.getOrDefault(emptyList()))

        // 实时通道: SSE, 断开后自动重连。
        launch {
            while (isActive) {
                try {
                    // 重连时补齐断线期间的消息。
                    merge(runCatching { chatApi.fetchHistory(roomId) }.getOrDefault(emptyList()))
                    chatApi.events(roomId).collect { event ->
                        if (event is WatchTogetherChatEvent.Chat) {
                            merge(listOf(event.message))
                        }
                    }
                } catch (e: CancellationException) {
                    throw e
                } catch (_: Exception) {
                    // SSE 不可用: 交给下面的轮询兜底, 稍后重试连接。
                }
                delay(SSE_RECONNECT_DELAY_MILLIS)
            }
        }

        // 安全网: 低频轮询, 覆盖 SSE 静默断开却未抛异常的情况。
        launch {
            while (isActive) {
                delay(POLL_INTERVAL_MILLIS)
                merge(runCatching { chatApi.fetchHistory(roomId) }.getOrDefault(emptyList()))
            }
        }
    }

    /**
     * 合并服务端消息与乐观上屏的本地消息。
     *
     * 服务端已回执的同 dedupKey 消息优先, 未回执的本地消息追加在末尾;
     * 乐观消息按 roomId 过滤, 避免切换房间时把上一个房间的气泡带过来。
     */
    private fun mergeWithPending(
        roomId: String,
        server: List<WatchTogetherChatMessage>,
        pending: List<WatchTogetherChatMessage>,
    ): List<WatchTogetherChatMessage> {
        val mine = pending.filter { it.roomId == roomId }
        if (mine.isEmpty()) return server
        val serverKeys = server.mapTo(HashSet()) { it.dedupKey }
        val stillPending = mine.filter { it.dedupKey !in serverKeys }
        return if (stillPending.isEmpty()) server else server + stillPending
    }

    /**
     * 把一条消息归属到官方房间成员: 官方成员列表是昵称与头像的权威来源,
     * 服务端只做透传与兜底(发言者可能已离开房间)。
     */
    private fun WatchTogetherChatMessage.withOfficialIdentity(
        members: List<WatchTogetherMemberIdentity>,
    ): WatchTogetherChatMessage {
        if (system) return this
        val member = members.firstOrNull { it.userId == userId } ?: return this
        return copy(nickname = member.nickname, avatarUrl = member.avatarUrl ?: avatarUrl)
    }

    /**
     * 聊天扩展链接, 空串表示未启用扩展。
     */
    val extensionUrl: StateFlow<String> = settingsRepository.watchTogetherSettings.flow
        .map { it.chatExtensionUrl }
        .stateInBackground("")

    fun onIntent(intent: WatchTogetherIntent) {
        when (intent) {
            is WatchTogetherIntent.JoinRoom -> launchInBackground {
                joinError.value = null
                manager.join(intent.roomName, intent.password)
                    .onFailure { throwable ->
                        joinError.value = (throwable as? WatchTogetherJoinException)?.failure?.name
                            ?: WatchTogetherJoinFailure.TEMPORARY.name
                    }
            }

            WatchTogetherIntent.LeaveRoom -> launchInBackground {
                joinError.value = null
                // 离开房间时清掉未回执的乐观消息, 避免残留到下一个房间。
                pendingOutgoing.value = emptyList()
                manager.leave()
            }

            is WatchTogetherIntent.SetFollowing -> launchInBackground {
                manager.setFollowing(intent.following)
            }

            WatchTogetherIntent.DisableFeature -> launchInBackground {
                joinError.value = null
                pendingOutgoing.value = emptyList()
                settingsRepository.watchTogetherSettings.update { copy(enabled = false) }
            }
        }
    }

    fun onPlayerEntryClick() {
        launchInBackground {
            val settings = settingsRepository.watchTogetherSettings.flow.first()
            if (!settings.enabled) {
                settingsRepository.watchTogetherSettings.update { copy(enabled = true) }
            }
            dialogOpenRequestChannel.send(Unit)
        }
    }

    fun onAppForegroundChanged(foreground: Boolean) {
        manager.setAppForeground(foreground)
    }

    /**
     * 在当前房间发送一条聊天消息。未加入房间时无操作。
     *
     * 采用**乐观上屏**: 立即把消息插入本地列表让 UI 马上显示, 再异步发往服务端;
     * 服务端回执(HTTP 响应或 SSE 推送)到达后按 clientMessageId 去重, 无缝替换本地消息。
     * 发送失败时移除本地消息, 避免留下永远发不出去的气泡。
     *
     * nonce 与 roomId 由 [WatchTogetherManager] 提供, UI 层不直接接触会话凭据。
     */
    fun sendChatMessage(content: String) {
        val trimmed = content.trim()
        if (trimmed.isEmpty()) return
        launchInBackground {
            val credentials = manager.currentRoomCredentials() ?: return@launchInBackground
            val sender = currentSender()
            val clientMessageId = "c_" + Random.nextLong().toString(16)

            // 1. 立即上屏, 让"点击发送"到"看到消息"之间没有网络等待。
            val optimistic = WatchTogetherChatMessage(
                id = clientMessageId,
                roomId = credentials.roomId,
                userId = sender?.userId ?: selfUserId.value.orEmpty(),
                nickname = sender?.nickname ?: "",
                avatarUrl = sender?.avatarUrl,
                content = trimmed,
                sentAt = manager.serverNowMillis(),
                clientMessageId = clientMessageId,
                pending = true,
            )
            pendingOutgoing.update { it + optimistic }

            // 2. 后台真正发送。HTTP 响应先到则立即替换, SSE 推送先到则由其替换。
            val sent = chatApi.send(
                roomId = credentials.roomId,
                sessionNonce = credentials.sessionNonce,
                content = trimmed,
                sender = sender,
                clientMessageId = clientMessageId,
            )
            if (sent == null) {
                // 发送失败: 撤下本地消息, 避免留下发不出去的气泡。
                pendingOutgoing.update { list -> list.filterNot { it.clientMessageId == clientMessageId } }
            } else {
                // 用服务端消息替换本地乐观消息(若 SSE 已先替换, 这里等于无操作)。
                pendingOutgoing.update { list ->
                    list.map { if (it.clientMessageId == clientMessageId) sent.copy(pending = false) else it }
                }
            }
        }
    }

    /**
     * 当前用户在官方房间里的成员信息; 即使房间成员列表尚未同步到本人,
     * 也会从 selfInfo 获取身份信息发送给服务端,确保身份稳定且正确显示。
     */
    private suspend fun currentSender(): WatchTogetherChatSender? {
        val selfId = selfUserId.value ?: return null
        // 优先从官方房间成员列表获取(最权威)
        val member = roomMemberIdentities.value.firstOrNull { it.userId == selfId }
        if (member != null) {
            return WatchTogetherChatSender(
                userId = member.userId,
                nickname = member.nickname,
                avatarUrl = member.avatarUrl,
            )
        }
        // 成员列表尚未同步时,从 selfInfo 获取基本信息
        val selfInfo = selfInfoProducer.flow.value.selfInfo ?: return null
        return WatchTogetherChatSender(
            userId = selfId,
            nickname = selfInfo.nickname.ifEmpty { selfInfo.bangumiUsername ?: "用户$selfId" },
            avatarUrl = selfInfo.avatarUrl,
        )
    }

    /**
     * 开启一起看功能(仅更新设置,不弹对话框)。
     */
    fun enableFeature() {
        launchInBackground {
            settingsRepository.watchTogetherSettings.update { copy(enabled = true) }
        }
    }

    /**
     * 设置聊天扩展链接。传空串表示停用扩展(不影响官方一起看房间)。
     */
    fun setExtensionUrl(address: String) {
        launchInBackground {
            settingsRepository.watchTogetherSettings.update { copy(chatExtensionUrl = address.trim()) }
        }
    }

    private fun RoomSession.presentationFlow(): Flow<RoomProjection> = combine(
        snapshot,
        connection,
        following,
        selfInfoProducer.flow,
        tickerFlow(),
    ) { snapshot, connection, following, selfInfo, _ ->
        val now = serverClock.now()
        val selfUserId = selfInfo.selfInfo?.id?.toString()
        RoomProjection(
            phase = WatchTogetherPhase.IN_ROOM,
            following = following,
            isSelfHost = isHost,
            room = WatchTogetherRoomCardState(
                roomName = roomName,
                connection = connection.toPresentation(),
                playback = snapshot.playback?.info?.toWatchTogetherPlaybackPresentation(now),
                members = snapshot.members
                    .sortedWith(compareByDescending { it.isHost })
                    .map { member ->
                        member.toWatchTogetherMemberPresentation(now, selfUserId)
                    },
            ),
        )
    }

    private fun tickerFlow(): Flow<Unit> = flow {
        while (true) {
            emit(Unit)
            delay(1_000L)
        }
    }

    private fun WatchTogetherConnectionState.toPresentation(): WatchTogetherConnectionPresentation = when (this) {
        WatchTogetherConnectionState.ConnectedSse -> WatchTogetherConnectionPresentation.CONNECTED
        WatchTogetherConnectionState.Reconnecting -> WatchTogetherConnectionPresentation.RECONNECTING
        WatchTogetherConnectionState.DegradedPolling -> WatchTogetherConnectionPresentation.DEGRADED
    }

    private data class RoomProjection(
        val phase: WatchTogetherPhase = WatchTogetherPhase.NOT_IN_ROOM,
        val room: WatchTogetherRoomCardState? = null,
        val following: Boolean = true,
        val isSelfHost: Boolean = false,
    )

    private companion object {
        /** 与服务端一致的消息保留上限。 */
        const val MAX_CHAT_HISTORY = 200

        /** 轮询安全网的间隔: 覆盖 SSE 静默断开却未抛异常的情况。 */
        const val POLL_INTERVAL_MILLIS = 5_000L

        /** SSE 断开后的重连间隔。 */
        const val SSE_RECONNECT_DELAY_MILLIS = 2_000L
    }
}
