#!/bin/sh

set -eu

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
repository_dir=$(CDPATH= cd -- "$script_dir/../../../.." && pwd)
frontend_dir="$repository_dir/frontend"
dist_dir="$script_dir/dist"
timeless_path="/timeless/0.31.4"
timeless_source_dir="$frontend_dir/public$timeless_path"
timeless_dist_dir="$dist_dir/assets$timeless_path"

rm -rf -- "$dist_dir"
mkdir -p -- "$timeless_dist_dir"

cp -- "$script_dir/public/index.html" "$dist_dir/index.html"
cp -- "$script_dir/public/style.css" "$dist_dir/style.css"
cp -- "$script_dir/public/app.js" "$dist_dir/app.js"
cp -- "$script_dir/worker.js" "$dist_dir/_worker.js"
cp -- "$timeless_source_dir/timeless.umd.min.js" "$timeless_dist_dir/timeless.umd.min.js"
cp -- "$timeless_source_dir/timeless.dom.umd.min.js" "$timeless_dist_dir/timeless.dom.umd.min.js"

printf 'Bridge Admin Pages assets prepared in %s\n' "$dist_dir"
