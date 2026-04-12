@echo off
setlocal
set PLAYFAST_SUDO=pass

REM 安装 Wails
go install github.com/wailsapp/wails/v2/cmd/wails@latest
REM 获取最新标签（按版本降序排序，并取第一个）
set "latest_tag="

for /f "delims=" %%i in ('git tag --sort=-v:refname') do (
    set "latest_tag=%%i"
    goto after_loop
)

:after_loop

if defined latest_tag (
    echo Latest Git Tags: %latest_tag%
) else (
    set "latest_tag=v1.0.0"
    echo Labels were not found in the repository. Use default tags: %latest_tag%
)

REM 构建项目
wails build -clean -ldflags "-s -w -X \"main.Version=%latest_tag%\"" -platform windows/amd64 -tags "with_gvisor,with_clash_api" -trimpath -webview2 embed
wails build -nsis -ldflags "-s -w -X \"main.Version=%latest_tag%\"" -platform windows/amd64 -tags "with_gvisor,with_clash_api" -trimpath -webview2 embed

REM 计算 SHA-256
set "file=build\bin\YuLiReBa.exe"
if exist "%file%" (
    echo SHA-256:
    CertUtil -hashfile "%file%" SHA256
) else (
    echo 找不到文件: %file%
)

endlocal