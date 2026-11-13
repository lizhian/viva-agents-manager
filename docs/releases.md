# 发布

推送 `vX.X.X` 标签后，`.github/workflows/release.yml` 自动测试、构建并发布六种目标：macOS、Windows、Linux × amd64、arm64。x86 指 x86_64（amd64），不提供 32 位 x86 包。

工作流从标签同步 `viva.toml` 与界面内嵌版本号，打包器版本取自 `go.mod` 的 Viva 依赖；macOS 在 Mac 上构建，Windows 与 Linux 在 Ubuntu 上交叉构建。各目标上传安装包与压缩包，全部 12 个文件齐备并通过 SHA256 校验后才发布 GitHub Release。失败的上传保留为草稿，可以重新运行工作流；已公开的 Release 不被改写。

可以通过 Actions 的手动入口传入版本号运行只构建、不发布的预检；只有推送版本标签才会发布 Release。

发布说明优先读取 `docs/releases/<版本>.md`，不存在时使用 GitHub 自动生成的说明。目前不配置签名、公证、更新源或包管理器发布。

本地预检：

```sh
python3 tools/release.py prepare v1.0.0
go vet ./...
go test ./... -count=1
go install github.com/vivaui/viva/cmd/viva@v0.6.1
viva package --target darwin --arch arm64 --format dmg,zip --no-sign -o 'bin/packages/Viva Agents Manager.app'
python3 tools/release.py verify v1.0.0 --target darwin --arch arm64 --dir bin/packages
```

`prepare` 会修改本地版本文件。标签应指向已提交且已推送的发布代码。
