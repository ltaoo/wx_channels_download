#!/usr/bin/env bash
set -euo pipefail

SOURCE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TEST_DIR="$(mktemp -d)"
trap 'rm -rf "$TEST_DIR"' EXIT

gcc -O2 -Wall -Wextra -Werror -fPIC -shared -o "$TEST_DIR/shim.so" "$SOURCE_DIR/hide-sve-sme.c" -ldl
gcc -O2 -Wall -Wextra -Werror -DWX_HWCAP_PROBE -o "$TEST_DIR/probe" "$SOURCE_DIR/hide-sve-sme.c"
gcc -O2 -Wall -Wextra -Werror -fPIC -shared -DWX_HWCAP_FIXTURE -o "$TEST_DIR/fixture.so" "$SOURCE_DIR/test.c" -ldl
gcc -O2 -Wall -Wextra -Werror -o "$TEST_DIR/check" "$SOURCE_DIR/test.c" -ldl

export TEST_HWCAP=0x2fb3ffff
export TEST_HWCAP2=0x1a03f986181
env LD_PRELOAD="$TEST_DIR/shim.so:$TEST_DIR/fixture.so" "$TEST_DIR/check" 0x2fb3ffff 0x186181
TEST_HWCAP2=0x7e13fb86181 env LD_PRELOAD="$TEST_DIR/shim.so:$TEST_DIR/fixture.so" "$TEST_DIR/check" 0x2fb3ffff 0x100386181
# All SVE/SME bits, including the high bits, are hidden; unrelated bits survive.
TEST_HWCAP=0xffffffffffffffff TEST_HWCAP2=0xffffffffffffffff \
    env LD_PRELOAD="$TEST_DIR/shim.so:$TEST_DIR/fixture.so" "$TEST_DIR/check" 0xffff001fffbfffff 0x81ffd80d807fe181

env LD_PRELOAD="$TEST_DIR/fixture.so" "$TEST_DIR/probe"
TEST_HWCAP2=0x7e13fb86181 env LD_PRELOAD="$TEST_DIR/fixture.so" "$TEST_DIR/probe"
expect_no_workaround() {
    local status=0
    env LD_PRELOAD="$TEST_DIR/fixture.so" "$TEST_DIR/probe" || status=$?
    test "$status" -eq 1
}
TEST_HWCAP=0x2ff3ffff expect_no_workaround # SVE and SME available
TEST_HWCAP2=0x186181 expect_no_workaround # Neither SVE nor SME
TEST_HWCAP2=0x2000000000 env LD_PRELOAD="$TEST_DIR/fixture.so" "$TEST_DIR/probe" # SME2 only

# Exercise the real init script against temporary paths, preserving other entries.
sed -e "s|/etc/ld.so.preload|$TEST_DIR/preload|g" \
    -e "s|/usr/local/lib/wx-hide-sve-sme\\\\.so|$TEST_DIR/shim\\\\.so|g" \
    -e "s|/usr/local/lib/wx-hide-sve-sme.so|$TEST_DIR/shim.so|g" \
    -e "s|/usr/local/libexec/wx-needs-sme-workaround|$TEST_DIR/probe|g" \
    "$SOURCE_DIR/../rootfs/custom-cont-init.d/10-wx-hwcap" > "$TEST_DIR/init"
mkdir "$TEST_DIR/bin"
printf '#!/bin/sh\necho aarch64\n' > "$TEST_DIR/bin/uname"
chmod +x "$TEST_DIR/bin/uname"
run_init() {
    env PATH="$TEST_DIR/bin:$PATH" LD_PRELOAD="$TEST_DIR/fixture.so" bash "$TEST_DIR/init"
}
run_init
test "$(grep -cFx "$TEST_DIR/shim.so" "$TEST_DIR/preload")" -eq 1
printf '/other-library.so' > "$TEST_DIR/preload" # Existing file may lack a final newline.
run_init
test "$(grep -cFx "$TEST_DIR/shim.so" "$TEST_DIR/preload")" -eq 1
grep -qFx /other-library.so "$TEST_DIR/preload"
run_init
test "$(grep -cFx "$TEST_DIR/shim.so" "$TEST_DIR/preload")" -eq 1
TEST_HWCAP=0x2ff3ffff run_init
test "$(sed '/^$/d' "$TEST_DIR/preload")" = /other-library.so
run_init
TEST_HWCAP2=0x186181 run_init
test "$(sed '/^$/d' "$TEST_DIR/preload")" = /other-library.so
printf '#!/bin/sh\necho x86_64\n' > "$TEST_DIR/bin/uname"
run_init
test "$(sed '/^$/d' "$TEST_DIR/preload")" = /other-library.so

echo "hwcap compatibility tests passed"
