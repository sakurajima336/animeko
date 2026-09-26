/*
 * Copyright (C) 2024-2026 OpenAni and contributors.
 *
 * 此源代码的使用受 GNU AFFERO GENERAL PUBLIC LICENSE version 3 许可证的约束, 可以在以下链接找到该许可证.
 * Use of this source code is governed by the GNU AGPLv3 license, which can be found at the following link.
 *
 * https://github.com/open-ani/ani/blob/main/LICENSE
 */

package me.him188.ani.app.ui.watchtogether

import kotlinx.coroutines.channels.Channel
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.combine
import kotlinx.coroutines.flow.distinctUntilChanged
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.flow.flatMapLatest
import kotlinx.coroutines.flow.flow
import kotlinx.coroutines.flow.flowOf
import kotlinx.coroutines.flow.map
import kotlinx.coroutines.flow.receiveAsFlow
import me.him188.ani.app.data.network.WatchTogetherChatApi
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
     */
    val chatMessages: StateFlow<List<WatchTogetherChatMessage>> = manager.state
        .map { (it as? WatchTogetherState.InRoom)?.session?.roomId }
        .distinctUntilChanged()
        .flatMapLatest { roomId ->
            if (roomId == null) {
                flowOf(emptyList<WatchTogetherChatMessage>())
            } else {
                // 每 3 秒拉一次历史,作为 SSE 之外的兜底(例如连接降级时)。
                flow {
                    while (true) {
                        emit(chatApi.fetchHistory(roomId))
                        delay(3_000L)
                    }
                }
            }
        }
        // 成员列表变化时只重新投影昵称/头像, 不打断上面的轮询。
        .combine(roomMemberIdentities) { messages, members ->
            messages.map { it.withOfficialIdentity(members) }
        }
        .stateInBackground(emptyList())
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
                manager.leave()
            }

            is WatchTogetherIntent.SetFollowing -> launchInBackground {
                manager.setFollowing(intent.following)
            }

            WatchTogetherIntent.DisableFeature -> launchInBackground {
                joinError.value = null
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
     * nonce 与 roomId 由 [WatchTogetherManager] 提供,UI 层不直接接触会话凭据。
     * 昵称与头像取自官方房间成员信息, 因此聊天室里的显示与官方成员列表一致。
     */
    fun sendChatMessage(content: String) {
        val trimmed = content.trim()
        if (trimmed.isEmpty()) return
        launchInBackground {
            val credentials = manager.currentRoomCredentials() ?: return@launchInBackground
            chatApi.send(
                roomId = credentials.roomId,
                sessionNonce = credentials.sessionNonce,
                content = trimmed,
                sender = currentSender(),
            )
        }
    }

    /**
     * 当前用户在官方房间里的成员信息; 房间成员列表尚未同步到本人时返回 null,
     * 由服务端沿用该 sessionNonce 已登记的身份。
     */
    private fun currentSender(): WatchTogetherChatSender? {
        val selfId = selfUserId.value ?: return null
        val member = roomMemberIdentities.value.firstOrNull { it.userId == selfId } ?: return null
        return WatchTogetherChatSender(
            userId = member.userId,
            nickname = member.nickname,
            avatarUrl = member.avatarUrl,
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
}
