#!/usr/bin/env bash
#
# Licensed to the Apache Software Foundation (ASF) under one
# or more contributor license agreements. See the NOTICE file
# distributed with this work for additional information
# regarding copyright ownership. The ASF licenses this file
# to you under the Apache License, Version 2.0 (the
# "License"); you may not use this file except in compliance
# with the License. You may obtain a copy of the License at
#
#   http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing,
# software distributed under the License is distributed on an
# "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
# KIND, either express or implied. See the License for the
# specific language governing permissions and limitations
# under the License.
#
# Runs both Go generators over the real-world IDL in repos.tsv and checks that
#   1. the C++ compiler accepts every file,
#   2. thrift-go accepts what the C++ compiler accepts and writes the same
#      bytes, apart from the entries in known-parity.txt,
#   3. the Go the C++ compiler writes for the largest services builds against
#      lib/go, apart from the packages in known-build.txt.
# Entries in the two known-*.txt files that no longer occur are reported so
# that the lists shrink as gaps close.
#
# usage: run.sh <thrift (C++)> <thrift-go> <work dir>
# Set UPDATE_KNOWN=1 to rewrite the two baselines from this run.

set -uo pipefail

here=$(cd "$(dirname "$0")" && pwd)
root=$(cd "$here/../../.." && pwd)
cpp=$(cd "$(dirname "$1")" && pwd)/$(basename "$1")
tgo=$(cd "$(dirname "$2")" && pwd)/$(basename "$2")
work=$3
mkdir -p "$work"
work=$(cd "$work" && pwd)
src=$work/src
out=$work/out
rm -rf "$out"
mkdir -p "$src" "$out"

# Fetch each repository at its pinned commit, thrift directories only.
while IFS=$'\t' read -r name repo sha paths; do
  [[ -z "$name" || "$name" == \#* ]] && continue
  d=$src/$name
  if [[ ! -d $d/.git ]]; then
    git init -q "$d"
    git -C "$d" remote add origin "https://github.com/$repo.git"
    git -C "$d" sparse-checkout set --no-cone $paths
  fi
  if [[ "$(git -C "$d" rev-parse -q --verify HEAD)" != "$sha" ]]; then
    git -C "$d" fetch -q --depth 1 --filter=blob:none origin "$sha" && git -C "$d" checkout -q FETCH_HEAD \
      || { echo "error: cannot fetch $repo@$sha" >&2; exit 2; }
  fi
done < "$here/repos.tsv"

# Files the projects' own builds provide: fb303 for the Hive metastore, and the
# ErrorCodes.thrift that Impala generates.
mkdir -p "$src/_inc/share/fb303/if"
cp "$root/contrib/fb303/if/fb303.thrift" "$src/_inc/share/fb303/if/"
(cd "$src/impala/common/thrift" && python3 generate_error_codes.py > /dev/null)
inc=(-I "$src/_inc" -I "$src/hive/service-rpc/if"
     -I "$src/hive/standalone-metastore/metastore-common/src/main/thrift"
     -I "$src/iotdb/iotdb-protocol/thrift-commons/src/main/thrift")

# Generate every file with both compilers.
: > "$work/results.tsv"
while read -r f; do
  rel=${f#"$src"/}
  proj=${rel%%/*}
  base=$(basename "$f" .thrift)
  for c in cpp go; do
    d=$out/$c/$proj/$base
    mkdir -p "$d"
    if [[ $c == cpp ]]; then bin=$cpp; else bin=$tgo; fi
    "$bin" -r --gen go -I "$(dirname "$f")" "${inc[@]}" -out "$d" "$f" > "$d.log" 2>&1
    printf '%s\t%s\t%s\n' "$c" "$rel" "$?" >> "$work/results.tsv"
  done
done < <(find "$src" -path "$src/_inc" -prune -o -name '*.thrift' -print | LC_ALL=C sort)

status=0
summary=$work/summary.md
total=$(awk -F'\t' '$1=="cpp"' "$work/results.tsv" | wc -l | tr -d ' ')
cpp_rej=$(awk -F'\t' '$1=="cpp" && $3!="0"{print $2}' "$work/results.tsv")
{
  echo "### Real-world IDL corpus"
  echo
  echo "- IDL files: $total from $(grep -cv '^#' "$here/repos.tsv") projects"
  echo "- rejected by the C++ compiler: $(printf '%s' "$cpp_rej" | grep -c .)"
} > "$summary"
if [[ -n "$cpp_rej" ]]; then
  echo "error: the C++ compiler rejects corpus files:" >&2
  echo "$cpp_rej" >&2
  status=1
fi

# Parity: every difference between the two output trees, as one line per path.
(cd "$out" && diff -rq cpp go 2>/dev/null) \
  | sed -E 's#^Files cpp/([^ ]+) and go/[^ ]+ differ$#differ \1#; s#^Only in (cpp|go)/?([^:]*): (.*)$#only-\1 \2/\3#' \
  | grep -v '\.log$' | LC_ALL=C sort > "$work/parity.txt"

# Build: the Go the C++ compiler writes for the largest services, against lib/go.
: > "$work/build.txt"
for spec in \
    hive/standalone-metastore/metastore-common/src/main/thrift/hive_metastore.thrift \
    hive/service-rpc/if/TCLIService.thrift \
    parquet-format/src/main/thrift/parquet.thrift \
    starrocks/gensrc/thrift/FrontendService.thrift \
    doris/gensrc/thrift/FrontendService.thrift \
    impala/common/thrift/ImpalaService.thrift \
    evernote-thrift/src/NoteStore.thrift \
    iotdb/iotdb-protocol/thrift-confignode/src/main/thrift/confignode.thrift; do
  name=$(basename "$spec" .thrift)
  m=$work/build/$name
  rm -rf "$m" && mkdir -p "$m"
  printf 'module example.com/%s\n\ngo 1.26\n\nrequire github.com/apache/thrift v0.0.0\n\nreplace github.com/apache/thrift => %s\n' "$name" "$root" > "$m/go.mod"
  "$cpp" -r --gen "go:package_prefix=example.com/$name/" -I "$(dirname "$src/$spec")" "${inc[@]}" -out "$m" "$src/$spec" > /dev/null 2>&1
  (cd "$m" && go mod tidy > /dev/null 2>&1
   for p in $(go list ./... 2>/dev/null); do
     go build "$p" > /dev/null 2>&1 || echo "${p#example.com/}"
   done) >> "$work/build.txt"
done
LC_ALL=C sort -o "$work/build.txt" "$work/build.txt"

if [[ "${UPDATE_KNOWN:-}" == 1 ]]; then
  { echo "# thrift-go vs C++ Go output differences on the corpus that are known and tracked."
    echo "# Regenerate with UPDATE_KNOWN=1 compiler/go/corpus/run.sh ..."
    cat "$work/parity.txt"; } > "$here/known-parity.txt"
  { echo "# Go packages generated by the C++ compiler that do not build against lib/go."
    echo "# Regenerate with UPDATE_KNOWN=1 compiler/go/corpus/run.sh ..."
    cat "$work/build.txt"; } > "$here/known-build.txt"
fi

check() { # <label> <this run> <baseline>
  local new gone
  new=$(LC_ALL=C comm -23 "$2" <(grep -v '^#' "$3" | LC_ALL=C sort))
  gone=$(LC_ALL=C comm -13 "$2" <(grep -v '^#' "$3" | LC_ALL=C sort))
  echo "- $1: $(grep -c . "$2") (new: $(printf '%s' "$new" | grep -c .), no longer seen: $(printf '%s' "$gone" | grep -c .))" >> "$summary"
  if [[ -n "$new" ]]; then
    echo "error: new $1:" >&2
    echo "$new" >&2
    status=1
  fi
  if [[ -n "$gone" ]]; then
    echo "note: $1 no longer seen, remove from $(basename "$3"):"
    echo "$gone"
  fi
}
check "thrift-go vs C++ differences" "$work/parity.txt" "$here/known-parity.txt"
check "Go packages that do not build" "$work/build.txt" "$here/known-build.txt"

cat "$summary"
[[ -n "${GITHUB_STEP_SUMMARY:-}" ]] && cat "$summary" >> "$GITHUB_STEP_SUMMARY"
exit $status
