#!/bin/sh

set -eu

if [ "$#" -lt 2 ] || [ "$1" != "--" ]; then
	echo "usage: $0 -- command [args...]" >&2
	exit 2
fi
shift

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd -P)
repository_root=$(CDPATH= cd -- "$script_dir/.." && pwd -P)
protected_relative_path="data/memory.db"
protected_absolute_path="$repository_root/$protected_relative_path"

normalize_path_argument() {
	candidate=$1
	case "$candidate" in
	*=*) candidate=${candidate#*=} ;;
	esac
	if [ -z "$candidate" ]; then
		return 1
	fi
	case "$candidate" in
	/*) absolute_candidate=$candidate ;;
	*) absolute_candidate=$repository_root/$candidate ;;
	esac
	candidate_directory=$(dirname -- "$absolute_candidate")
	candidate_name=$(basename -- "$absolute_candidate")
	normalized_directory=$(CDPATH= cd -- "$candidate_directory" 2>/dev/null && pwd -P) || return 1
	printf '%s/%s\n' "$normalized_directory" "$candidate_name"
}

is_protected_path() {
	candidate=$1
	for protected_path in "$protected_absolute_path" "$protected_absolute_path-wal" "$protected_absolute_path-shm"; do
		if [ "$candidate" = "$protected_path" ]; then
			return 0
		fi
		if [ -e "$candidate" ] && [ -e "$protected_path" ] && command -v realpath >/dev/null 2>&1; then
			candidate_target=$(realpath "$candidate")
			protected_target=$(realpath "$protected_path")
			if [ "$candidate_target" = "$protected_target" ]; then
				return 0
			fi
		fi
	done
	return 1
}

for argument in "$@"; do
	if normalized_argument=$(normalize_path_argument "$argument") && is_protected_path "$normalized_argument"; then
		echo "refusing command that references the real database: $argument" >&2
		exit 2
	fi
done

before=$(mktemp "${TMPDIR:-/tmp}/memory-db-metadata-before.XXXXXX")
after=$(mktemp "${TMPDIR:-/tmp}/memory-db-metadata-after.XXXXXX")
trap 'rm -f "$before" "$after"' EXIT HUP INT TERM

record_metadata() {
	destination=$1
	for path in "$protected_absolute_path" "$protected_absolute_path-wal" "$protected_absolute_path-shm"; do
		if [ ! -e "$path" ]; then
			printf '%s|missing\n' "$path" >>"$destination"
		elif stat -f '%N|present|%i|%z|%p|%Fm|%Fc' "$path" >/dev/null 2>&1; then
			stat -f '%N|present|%i|%z|%p|%Fm|%Fc' "$path" >>"$destination"
		else
			stat -c '%n|present|%i|%s|%f|%y|%z' "$path" >>"$destination"
		fi
	done
}

record_metadata "$before"
status=0
"$@" || status=$?
record_metadata "$after"

if ! diff -u "$before" "$after"; then
	echo "real database filesystem metadata changed" >&2
	exit 1
fi

echo "real database filesystem metadata unchanged"
exit "$status"
