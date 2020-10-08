#!/bin/sh
set -eu

outdir_name="genproto"

usage() {
    printf 'usage: %s --boostrap|PATH_TO_TENSORBOARD_REPO\n' "$0"
    printf '\n'
    printf 'With --bootstrap, defines Go modules so that go.mod parses.\n'
    printf 'With TensorBoard repository, replaces directory "%s" with\n' \
        "${outdir_name}"
    printf 'Go bindings for proto definitions.\n'
}

needs() {
    failed=
    for arg; do
        if ! command -v "${arg}" >/dev/null 2>&1; then
            failed=1
            printf 'error: %s: command not found\n' "${arg}"
        fi
    done
    if [ -n "${failed}" ]; then
        printf >&2 'fatal: install dependencies and try again\n'
        exit 1
    fi
}

main() {
    if [ $# -ne 1 ]; then
        usage >&2
        exit 1
    fi
    outdir="$PWD/${outdir_name}"
    case "$1" in
        --help) usage ;;
        --bootstrap) bootstrap ;;
        --*)
            usage >&2
            exit 1
            ;;
        *)
            if ! [ -d "$1" ]; then
                usage >&2
                exit 1
            fi
            compile "$1"
            ;;
    esac
}

bootstrap() (
    mkdir -p "${outdir}"
    cd "${outdir}"
    for module in \
        github.com/tensorflow/tensorflow \
        github.com/wchargin/tensorboard-data-server/proto \
    ; do
        mkdir -p "${module}"
        if ! [ -f "${module}/go.mod" ]; then
            (cd "${module}" && go mod init "${module}")
        fi
    done
)

compile() {
    needs protoc protoc-gen-go protoc-gen-go-grpc
    case "$1" in
        /*) tensorboard_repo="$1" ;;
        *) tensorboard_repo="$PWD/$1" ;;
    esac
    rm -rf "${outdir}"
    mkdir -p "${outdir}"
    (
        cd "${tensorboard_repo}"
        find tensorboard/compat/proto/ -name '*.proto' \
            -exec protoc --go_out="${outdir}" --go_opt=paths=import {} +
    )
    (
        cd ./proto
        find . -name '*.proto' -exec protoc \
            -I"${tensorboard_repo}" -I. \
            --go_out="${outdir}" --go-grpc_out="${outdir}" \
            --go_opt=paths=import --go-grpc_opt=paths=import \
            {} +
    )
    bootstrap
}

main "$@"
