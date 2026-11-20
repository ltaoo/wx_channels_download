#!/bin/sh
set -eu

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
script=$script_dir/disable-wechat-liteapp-finder.sh
test_dir=$(mktemp -d "${TMPDIR:-/tmp}/wx-finder-switch-test.XXXXXX")
trap 'rm -rf "$test_dir"' 0
trap 'exit 1' HUP INT TERM
users_root=$test_dir/users
relative=xworker/liteapp/local/packages/ilinkres_uzhblidb/engineConfig

make_config() {
    config=$users_root/$1/$relative/$2/dist/engineConfig.json
    mkdir -p "$(dirname "$config")"
    printf '{"featureSwitch":{"handleFinder":%s,"keep":true},"msgAppIdWhiteList":[{"appId":"fixture"}],"other":{"name":"中文"}}\n' "$3" > "$config"
}
make_config 'user one' 8 false
make_config 'user one' 10 true
make_config 'user two' 2 true
make_config 'user two' not-a-version true
cp "$users_root/user one/$relative/8/dist/engineConfig.json" "$test_dir/original-false.json"
sh "$script" "$users_root" > "$test_dir/first.log"

for config in "$users_root/user one/$relative/10/dist/engineConfig.json" "$users_root/user two/$relative/2/dist/engineConfig.json"; do
    [ "$(plutil -extract featureSwitch.handleFinder raw -expect bool "$config")" = false ]
    [ "$(plutil -extract featureSwitch.keep raw -expect bool "$config")" = true ]
    [ "$(plutil -extract msgAppIdWhiteList.0.appId raw "$config")" = fixture ]
    [ "$(plutil -extract other.name raw "$config")" = 中文 ]
    set -- "$config".bak.*
    [ "$#" -eq 1 ] && [ -f "$1" ]
    [ "$(plutil -extract featureSwitch.handleFinder raw -expect bool "$1")" = true ]
done
cmp "$test_dir/original-false.json" "$users_root/user one/$relative/8/dist/engineConfig.json"
[ "$(plutil -extract featureSwitch.handleFinder raw "$users_root/user two/$relative/not-a-version/dist/engineConfig.json")" = true ]
config="$users_root/user one/$relative/10/dist/engineConfig.json"
before=$(cksum "$config")
sh "$script" "$users_root" > "$test_dir/second.log"
[ "$before" = "$(cksum "$config")" ]
set -- "$config".bak.*
[ "$#" -eq 1 ]

users_root=$test_dir/invalid
make_config invalid-user 1 true
printf '{"featureSwitch":{"handleFinder":"true"}}\n' > "$config"
cp "$config" "$test_dir/invalid-original.json"
if sh "$script" "$users_root" > "$test_dir/invalid.log" 2>&1; then
    printf 'FAIL: accepted a string instead of a boolean\n' >&2
    exit 1
fi
cmp "$config" "$test_dir/invalid-original.json"
printf '{invalid json\n' > "$config"
cp "$config" "$test_dir/malformed-original.json"
if sh "$script" "$users_root" > "$test_dir/malformed.log" 2>&1; then
    printf 'FAIL: accepted malformed JSON\n' >&2
    exit 1
fi
cmp "$config" "$test_dir/malformed-original.json"
mkdir "$test_dir/empty"
if sh "$script" "$test_dir/empty" > "$test_dir/empty.log" 2>&1; then
    printf 'FAIL: accepted an empty users directory\n' >&2
    exit 1
fi
printf 'PASS: discovery, backups, unrelated fields, repeated runs and invalid configs\n'
