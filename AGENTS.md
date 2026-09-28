# Repository Guidelines

This is the repository for the app. For the server, you can navigate to ../ani-api-server

Read docs/contributing for project guidelines. Before modifying a subsystem, check docs/contributing/code/ for its documentation (e.g. the media framework docs cover terminology, class-level code maps, and the playback flow) and read the relevant docs first.

## 分支职责

本仓库有两个长期分支，改动范围严格区分，不要把其中一个分支的内容带到另一个分支：

- **`main`** —— 与官方上游 `open-ani/animeko` 保持一致，只额外携带 fork 自己添加的功能（一起看聊天扩展、`watch-together-server/`）。**不得**包含任何 ARM64 专属的构建改动，这样任何人都能直接克隆构建。
- **`linux-arm64`** —— 在 `main` 之上，额外携带两处 Linux ARM64 构建改动（`settings.gradle.kts`、`torrent/anitorrent/build.gradle.kts`），用于在 ARM64 机器上启用自建的 anitorrent 原生库。该库不走 Maven Central，而是从本机仓库解析（`~/.m2` 或 `ani.anitorrent.localNativesRepo` 指定的路径），因此这些改动依赖机器本地的自建产物，其他人拉取后若没有该库会在配置阶段直接失败，**不要**合回 `main`。

同步上游时，先让 `main` 跟进上游，再把 `main` 合入 `linux-arm64`。分支差异、一次性准备与验证步骤见 `docs/linux-arm64.md`。

Additional requirements:

- You should add imports, instead of using fully qualified names in code.
- For Android Instrumented tests, you can just use `@Test`, no need to write `@RunWith` to the class.
- 注释和文档应直接描述当前设计、职责、行为与约束，不要用“不再…”“改为…”“新设计…”等措辞叙述开发过程，也不要记录未上线方案、被纠正的错误假设或对话历史。只有在解释兼容性或迁移逻辑确有必要时，才说明已发布版本的历史行为。

## UI Verification

- **Prefer reusable interactive UI tests** over driving a real window: use `runAniComposeUiTest` (`utils/ui-testing`) with synthetic input (`performClick`, `performTextInput`, `sendKeyEvent`) and assert on semantics — focus, text, state, bounds. They run without OS input — no focus stealing, no real mouse — and stay in the repo as regression tests. When you verify a UI change manually, consider leaving such a test behind.
- `assertScreenshot` compares against golden images only on desktop; on Android it is a no-op. Android and TV device tests do not capture or compare pixels: expose visual state that semantics cannot express (blur, dim, glow, whether an animation runs) through `TvVisualSemantics` in TV code, and test color or geometry calculations as pure functions in host tests. Check the rendered look manually with the skills below.
- Reserve the skills below for what headless tests cannot cover: JCEF, VLC/mpv playback, native libraries, packaging, window chrome, emulator behavior.

## Agent Skills

- For interactive Android UI verification (start emulator, install the app, then tap/swipe/type and verify via screenshots, UI-hierarchy dumps, logcat, and Figma design comparison), use the repo-local skill at `.agents/skills/android-ui-verify/SKILL.md`. Its toolbox script is `.agents/skills/android-ui-verify/scripts/droid.sh` (run `droid.sh help`). `.claude/skills/android-ui-verify` is a symlink to it for Claude Code auto-discovery.
- For desktop/PC executable validation (Compose Desktop, JCEF, VLC/native libraries, packaging, macOS window screenshots), use the repo-local skill at `.agents/skills/desktop-ui-verify/SKILL.md`.

## Generating Client

If you change server API, you can then use `./gradlew generateOpenApiForAnimeko` to automatically re-generate the client. Don't manually write http client.
