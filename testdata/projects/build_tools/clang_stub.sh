#!/bin/sh
set -eu
output=
while [ "$#" -gt 0 ]; do
    if [ "$1" = "-o" ]; then
        shift
        output=$1
    fi
    shift
done
case "$output" in
    */reject/*) exit 1 ;;
esac
if [ -n "${SEC_TEST_EDIT_MANIFEST:-}" ]; then
    printf '\n# external editor\n' >> "$SEC_TEST_EDIT_MANIFEST"
fi
printf 'linked fixture\n' > "$output"
