#!/bin/sh

set -eu

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd -P)
guard="$script_dir/verify-real-db-untouched.sh"

expect_rejected() {
	description=$1
	shift
	if "$guard" -- "$@" >/dev/null 2>&1; then
		echo "guard accepted $description" >&2
		exit 1
	fi
}

expect_rejected "a repeated-slash alias" printf '%s\n' 'data//memory.db'
expect_rejected "a dot-segment alias" printf '%s\n' './data/../data/memory.db'
expect_rejected "an environment assignment alias" env 'MEMORY_DB_PATH=data//memory.db' true
expect_rejected "an embedded protected path" printf '%s\n' 'prefix data/memory.db suffix'

for shell_interpreter in sh bash zsh; do
	if command -v "$shell_interpreter" >/dev/null 2>&1; then
		expect_rejected "$shell_interpreter indirection" "$shell_interpreter" -c 'printf "%s\n" data/memory.db >/dev/null'
	fi
done
expect_rejected "env shell indirection" env sh -c 'printf "%s\n" data/memory.db >/dev/null'

"$guard" -- true >/dev/null
sh -n "$guard"
echo "verify-real-db-untouched guard tests passed"
