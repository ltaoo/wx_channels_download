#!/bin/sh
# macOS：关闭本地 LiteApp 引擎对视频号的接管；可传入其他 radium/users 目录。
set -eu

if [ "$#" -gt 1 ]; then
    printf '用法：sh %s [radium/users 目录]\n' "$0" >&2
    exit 1
fi
users_root=${1:-"$HOME/Library/Containers/com.tencent.xinWeChat/Data/Documents/app_data/radium/users"}
if [ ! -d "$users_root" ]; then
    printf '未找到微信用户目录：%s\n' "$users_root" >&2
    exit 1
fi
if [ ! -x /usr/bin/plutil ]; then
    printf '需要 macOS 自带的 /usr/bin/plutil。\n' >&2
    exit 1
fi

temporary_file=
trap 'if [ -n "$temporary_file" ]; then rm -f "$temporary_file"; fi' 0
trap 'exit 1' HUP INT TERM
found_count=0
changed_count=0

for config in "$users_root"/*/xworker/liteapp/local/packages/ilinkres_uzhblidb/engineConfig/*/dist/engineConfig.json; do
    [ -f "$config" ] || continue
    if [ -L "$config" ]; then
        printf '跳过符号链接：%s\n' "$config" >&2
        continue
    fi
    version_dir=${config%/dist/engineConfig.json}
    version=${version_dir##*/}
    case "$version" in ''|*[!0-9]*) continue ;; esac
    found_count=$((found_count + 1))
    if ! state=$(/usr/bin/plutil -extract featureSwitch.handleFinder raw -expect bool -o - "$config"); then
        printf '无法读取布尔配置 featureSwitch.handleFinder：%s\n' "$config" >&2
        exit 1
    fi
    if [ "$state" = false ]; then
        printf '已为 false：%s\n' "$config"
        continue
    fi

    backup_file=$(mktemp "$config.bak.XXXXXX")
    cp -p "$config" "$backup_file"
    temporary_file=$(mktemp "$config.tmp.XXXXXX")
    cp -p "$backup_file" "$temporary_file"
    /usr/bin/plutil -replace featureSwitch.handleFinder -bool false "$temporary_file"
    [ "$(/usr/bin/plutil -extract featureSwitch.handleFinder raw -expect bool -o - "$temporary_file")" = false ]
    if ! cmp -s "$config" "$backup_file"; then
        printf '配置已被其他程序修改，停止写入：%s\n' "$config" >&2
        exit 1
    fi
    mv -f "$temporary_file" "$config"
    temporary_file=
    changed_count=$((changed_count + 1))
    printf '已改为 false：%s\n备份：%s\n' "$config" "$backup_file"
done

if [ "$found_count" -eq 0 ]; then
    printf '未找到引擎配置，请先在微信中打开一次视频号。搜索目录：%s\n' "$users_root" >&2
    exit 1
fi
printf '检查了 %s 个配置，修改了 %s 个。请完全退出并重新打开微信。\n' "$found_count" "$changed_count"
