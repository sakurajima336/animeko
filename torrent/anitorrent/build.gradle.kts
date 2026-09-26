/*
 * Copyright (C) 2024-2026 OpenAni and contributors.
 *
 * 此源代码的使用受 GNU AFFERO GENERAL PUBLIC LICENSE version 3 许可证的约束, 可以在以下链接找到该许可证.
 * Use of this source code is governed by the GNU AGPLv3 license, which can be found at the following link.
 *
 * https://github.com/open-ani/ani/blob/main/LICENSE
 */

plugins {
    id("ani.kmp-library")
    alias(libs.plugins.kotlin.plugin.serialization)

    // alias(libs.plugins.kotlinx.atomicfu)
}

kotlin {
    android {
        namespace = "me.him188.ani.torrent.anitorrent"
    }
    sourceSets.commonMain.dependencies {
        api(libs.kotlinx.coroutines.core)
        implementation(libs.kotlinx.collections.immutable)
        api(projects.utils.io)
        api(projects.utils.platform)
        api(projects.torrent.torrentApi)
        api(projects.utils.coroutines)
        implementation(libs.atomicfu)
    }
    sourceSets.getByName("jvmMain").dependencies {
        api(anitorrentLibs.anitorrent.native)
    }
    sourceSets.getByName("desktopMain").dependencies {
        val triple = getAnitorrentTriple()
        if (triple != null) {
            api(
                anitorrentLibs.anitorrent.native.desktop.asProvider().map { notation ->
                    "$notation:${triple}"
                },
            )
        }
    }
}

fun getAnitorrentTriple(): String? {
    return when (getOs()) {
        Os.MacOS -> {
            when (getArch()) {
                Arch.X86_64 -> "macos-x64"
                Arch.AARCH64 -> "macos-aarch64"
            }
        }

        Os.Windows -> {
            when (getArch()) {
                Arch.X86_64 -> "windows-x64"
                // Windows ARM64 builds intentionally ship without the local BT engine.
                Arch.AARCH64 -> null
            }
        }

        Os.Linux -> {
            when (getArch()) {
                Arch.X86_64 -> "linux-x64"
                // 上游未发布 Linux ARM64 的原生运行时 (anitorrent-native-desktop:linux-aarch64),
                // 因此默认不声明依赖 —— 声明了在配置阶段就会解析失败, 从而中断整个构建.
                // 自行编译并放进 ani.anitorrent.localNativesRepo 后, 用
                // `local.properties: ani.anitorrent.linux-aarch64=true` 打开.
                Arch.AARCH64 -> if (linuxAarch64NativeEnabled) "linux-aarch64" else null
            }
        }

        Os.Unknown -> error("Unsupported OS: ${getOs()}")
    }
}

/**
 * Linux ARM64 是否声明 anitorrent 原生运行时依赖.
 *
 * 默认关闭, 因为上游没有对应的产物; 打开前需要先准备好本地仓库里的自建原生 jar.
 */
val Project.linuxAarch64NativeEnabled: Boolean
    get() = getLocalProperty("ani.anitorrent.linux-aarch64")?.toBooleanStrictOrNull() ?: false

