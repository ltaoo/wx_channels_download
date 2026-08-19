#!/bin/sh

set -eu

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
frontend_dir=$(CDPATH= cd -- "$script_dir/../.." && pwd)
dist_dir="$script_dir/dist"
timeless_version="0.31.2"
timeless_source_dir="$frontend_dir/public/timeless/$timeless_version"
timeless_dist_dir="$dist_dir/assets/timeless/$timeless_version"

rm -rf -- "$dist_dir"
mkdir -p -- "$timeless_dist_dir"

cp -- "$script_dir/public/index.html" "$dist_dir/index.html"
cp -- "$script_dir/public/style.css" "$dist_dir/style.css"
cp -- "$script_dir/public/app.js" "$dist_dir/app.js"
cp -- "$timeless_source_dir/timeless.umd.min.js" "$timeless_dist_dir/timeless.umd.min.js"
cp -- "$timeless_source_dir/timeless.dom.umd.min.js" "$timeless_dist_dir/timeless.dom.umd.min.js"

printf 'Hub Admin Pages assets prepared in %s\n' "$dist_dir"
