#!/usr/bin/env bash
# Reproduces the build/footprint numbers in BENCHMARKS.md.
#
#   bash scripts/bench.sh
#
# Cold builds use a throwaway GOCACHE so your real build cache is untouched.
# Runtime numbers (startup, idle CPU, FPS) come from the JAMB_TRACE and
# JAMB_STRESS hooks; see BENCHMARKS.md for the exact commands.
set -euo pipefail

root="$(cd "$(dirname "$0")/.." && pwd)"
out="$(mktemp -d)"
trap 'rm -rf "$out"' EXIT
cd "$root/examples/demo"

ms() { echo $(( ($2 - $1) / 1000000 )); }
now() { date +%s%N; }

echo "== toolchain"
go version

echo "== cold build (empty GOCACHE, modules already downloaded)"
export GOCACHE="$out/gocache"
t0=$(now); go build -o "$out/demo.exe" .; t1=$(now)
echo "cold build: $(ms $t0 $t1) ms"
echo "fresh GOCACHE size after cold build: $(du -sh "$GOCACHE" | cut -f1)"

echo "== no-op rebuild"
t0=$(now); go build -o "$out/demo.exe" .; t1=$(now)
echo "no-op rebuild: $(ms $t0 $t1) ms"

edit_and_build() { # file, label
  local f="$1" label="$2"
  cp "$f" "$out/backup.go"
  printf '\nvar _ = "bench %s"\n' "$(now)" >> "$f"
  t0=$(now); go build -o "$out/demo.exe" .; t1=$(now)
  cp "$out/backup.go" "$f"
  echo "$label: $(ms $t0 $t1) ms"
}

echo "== incremental rebuilds (append a declaration, rebuild, restore)"
for i in 1 2 3; do edit_and_build views/projects.go "edit examples/demo/views/projects.go"; done
for i in 1 2 3; do edit_and_build ../../ui/theme.go "edit ui/theme.go"; done
for i in 1 2 3; do edit_and_build ../../internal/layout/layout.go "edit internal/layout/layout.go"; done
go build -o "$out/demo.exe" . # restore cache state

echo "== binary size"
go build -o "$out/demo.exe" .
echo "default:            $(du -h "$out/demo.exe" | cut -f1) ($(stat -c %s "$out/demo.exe") bytes)"
go build -trimpath -ldflags "-s -w" -o "$out/demo-small.exe" .
echo "-trimpath -s -w:    $(du -h "$out/demo-small.exe" | cut -f1) ($(stat -c %s "$out/demo-small.exe") bytes)"
go build -trimpath -ldflags "-s -w -H windowsgui" -o "$out/demo-gui.exe" . 2>/dev/null &&
  echo "release (windowsgui): $(stat -c %s "$out/demo-gui.exe") bytes"

echo "== source footprint"
echo "repository (excluding .git): $(du -sh --exclude=.git "$root" | cut -f1)"
echo "Go source lines: framework=$(cat $(find "$root" -path "$root/examples" -prune -o -name '*.go' -print) | wc -l) demo=$(cat $(find "$root/examples/demo" -name '*.go') | wc -l)"
modcache="$(go env GOMODCACHE)"
for m in golang.org/x/image@v0.46.0 golang.org/x/text@v0.42.0 golang.org/x/sys@v0.48.0; do
  [ -d "$modcache/$m" ] && echo "module cache $m: $(du -sh "$modcache/$m" | cut -f1)"
done
