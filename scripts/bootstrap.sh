#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail
shopt -s nullglob

usage() { # [CODE]
	local cmd="${0##*/}"
	cat <<-EOF
		Install package dependencies

		Usage:
		  $cmd [-h]

		Options:
		  -h  Show this message and exit.
	EOF
	exit "${1:-2}"
}

main() {
	local opt
	while getopts :h opt "$@"; do
		case "$opt" in
			h) usage 0 ;;
			*) echo "unknown option: $OPTARG" >&2 && usage ;;
		esac
	done
	shift $(( OPTIND-1 ))

	go mod download
	go generate ./...
}

main "$@"
