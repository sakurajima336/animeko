# Linux ARM64 开发分支

`main` 与官方上游保持一致，可正常构建。本分支（`linux-arm64`）额外携带**两处** Linux ARM64 专属改动，用于在 ARM64 机器上启用 anitorrent 本地 BT 引擎。

这两处改动**不要**合回 `main`：它们依赖机器本地的自建原生库，其它人拉取后若没有该库会在配置阶段直接失败。

## 分支差异

与 `main` 的差异只有两个文件：

| 文件 | 改动 |
|---|---|
| `settings.gradle.kts` | 新增 `anitorrentLocalNativesRepository()`，独占解析 `anitorrent-native-desktop` |
| `torrent/anitorrent/build.gradle.kts` | `getAnitorrentTriple()` 支持 `linux-aarch64`，由 `ani.anitorrent.linux-aarch64` 开关控制 |

## 为什么需要

上游 [open-ani/anitorrent](https://github.com/open-ani/anitorrent) 只发布 `linux-x64` / `macos-x64` / `macos-aarch64` / `windows-x64`，**没有** Linux ARM64 产物（README 的 Supported targets 亦如此声明）。

`anitorrent-native-desktop` 的 Gradle Module Metadata 里只登记了上游自己发布的 classifier，因此即使把自建的 `linux-aarch64` jar 放进 `~/.m2`，Gradle 也**不会**去找它 —— 模块元数据已被 `mavenCentral` 命中，解析路径里根本没有这个文件。

解法是让该模块**独占**走本地仓库：元数据用从 Central 取回的同一份 `.module`，自建的原生 jar 因此可达。

## 一次性准备

### 1. 系统依赖

```bash
sudo apt-get install -y cmake ninja-build swig libssl-dev pkg-config openjdk-21-jdk
```

> `openjdk-21-jdk-headless` 不含 `jawt.h` / `libjawt.so`，CMake 的 `find_package(JNI)` 会失败，必须装完整 `jdk` 包。

### 2. 编译原生库

```bash
git clone --depth 1 https://github.com/sakurajima336/anitorrent.git   # fork，含 linux-arm64 支持
cd anitorrent/anitorrent-native
export JAVA_HOME=/usr/lib/jvm/java-21-openjdk-arm64
cmake -DCMAKE_BUILD_TYPE=Release -DCMAKE_C_FLAGS_RELEASE=-O3 -G Ninja -S . -B build-ci
cmake --build build-ci --target anitorrent -j$(nproc)
```

产物为 `build-ci/libanitorrent.so`（约 13 MB，ARM aarch64）。首次构建需拉取 libtorrent v2.0.10 并编译 Boost，约 15 分钟。

### 3. 发布到本地仓库

把 `libanitorrent.so` 打成 classifier jar，连同从 Central 取回的元数据一起放进 `~/.m2/repository/org/openani/anitorrent/`：

```
anitorrent-native-desktop/0.2.0/
    anitorrent-native-desktop-0.2.0.module
    anitorrent-native-desktop-0.2.0.pom
    anitorrent-native-desktop-0.2.0.jar
    anitorrent-native-desktop-0.2.0-linux-aarch64.jar   # 自建
anitorrent-native/0.2.0/{*.module,*.pom,*.jar}
anitorrent-native-desktop-jni/0.2.0/{*.module,*.pom,*.jar}
```

### 4. 打开开关

`local.properties`（已被 gitignore）：

```properties
ani.anitorrent.linux-aarch64=true
```

可选：用 `ani.anitorrent.localNativesRepo` 指定非默认的本地仓库路径。

## 验证

```bash
./gradlew :app:desktop:run   # 带 ANIMEKO_DESKTOP_TEST_TASK=anitorrent-load-test
```

日志应出现：

```
AnitorrentLibraryLoader: Loading anitorrent library: success (from resources)
```

或直接检查原生库：

```bash
java -cp <anitorrent-native-desktop-jni.jar> ...   # anitorrentJNI.lt_version() -> "2.0.10.0"
```

## 与上游同步

`main` 正常跟进官方上游；本分支定期 rebase：

```bash
git checkout main && git fetch upstream && git merge --ff-only upstream/main && git push origin main
git checkout linux-arm64 && git rebase main && git push --force-with-lease origin linux-arm64
```

若 `settings.gradle.kts` 或 `torrent/anitorrent/build.gradle.kts` 在上游也有改动，rebase 时解决冲突即可 —— 本分支只占这两个文件的一小部分。
