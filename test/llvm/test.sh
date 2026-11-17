#!/bin/bash
# Copyright 2025 ByteDance Ltd. and/or its affiliates. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.

DIR=$(cd "$(dirname "$0")";pwd)

BEAST=bce,inlbudget=240,gabinline,intrinsic

if [[ $# -eq 1 ]]
then
  GCFLAGS="-beast=$BEAST,llvm=$1 -d panic=1 -c 1"
elif [[ $# -eq 2 ]]
then
  GCFLAGS="-beast=$BEAST,llvm=$1,optimize=$2 -d panic=1 -c 1"
elif [[ $# -eq 3 ]]
then
  GCFLAGS="-beast=$BEAST,llvm=$1,optimize=$2,inline=$3 -d panic=1 -c 1"
else
  echo "Too few or many arguments provided"
  exit 1
fi

TESTS=(slice quicksort intglobal \
    complex copy idata multiexit selectnaddr \
    atomic \
    reflect \
    typed \
    hmul \
    fma \
    libm \
    stackret \
    stackpara \
    closure \
    floatconst \
    emptyconst \
    map \
    strings \
    useitfc \
    pointer \
    stackalign \
    blockdefer \
    unnamedstruct \
    interface \
    channel \
    mutex \
    inlinestack \
    memset \
    panicindefer \
    panic1 \
    panic2 \
    panic3 \
    panic4 \
    panic5 \
    panic6 \
    stackdefer \
    panicatend \
    nilerror \
    nilmaparg \
    conststr \
    rsh8u64 \
    hmul64u \
    add \
    addptr
    and \
    arg \
    argintreg \
    argfloatreg
    cmpiter \
    arraymake \
    ttuple \
    avg64u \
    bitlen \
    bitrev \
    ceil \
    com \
    convert \
    unsafeconvert_keepalive \
    ctz \
    div128u \
    eqptr \
    floor \
    typenotmatch \
    compilepanic \
    move \
    mul64uhilo \
    popcount \
    rotateleft \
    roundtoeven \
    sqrt \
    sub64borrow \
    trunc \
    bpsp \
    shl \
    gcparamout \
    recurtype \
    tanh \
    duffcopy \
    fcmp \
    typecheck \
    machoreloc \
    zerosizestackobj \
    stackgcmerge \
    goavg64 \
    returnbool \
    packageinit \
    opendefergc \
    tailcallgc \
)

DIRTESTS=(linkname abi)
ROOTTESTS=(stackobj3_llvm)

if [[ "$1" == "clean" ]]; then
    rm -rf "${DIR}/llvm.o"
    rm -rf "${DIR}/llvm.ll"
    rm -rf "${DIR}/llvm.bc"
    rm -rf "${DIR}/ssa.html"
    for T in "${TESTS[@]}"
    do
        TBIN_PATH=$DIR/$T
        # // remove old bin
        rm -rf "$TBIN_PATH"
    done
    for T in "${DIRTESTS[@]}"
    do
        TBIN_PATH=$DIR/$T/$T
        rm -rf "$TBIN_PATH"
        rm -rf "${DIR}/$T/llvm.o"
        rm -rf "${DIR}/$T/llvm.ll"
        rm -rf "${DIR}/$T/llvm.bc"
    done
    exit 0
fi

# uncomment if u want to run `slice' only
#TESTS=(fma)
#UNITTEST=()


GO_BIN=${DIR}/../../bin/go

RESULTS=()
FAILED_TO_RUN_TESTS=()

# Isolate this run without invalidating the user's or concurrent builds' cache.
test_cache=$(mktemp -d "${TMPDIR:-/tmp}/tangollvm-tests.XXXXXX") || exit 1
export GOCACHE="$test_cache/cache"
mkdir -p "$test_cache/bin" || exit 1
trap 'rm -rf "$test_cache"' EXIT

for T in "${TESTS[@]}"
do
    TBIN_PATH=$test_cache/bin/$T

    test_gcflags=(-gcflags all="$GCFLAGS")
    if [[ "$T" == "unsafeconvert_keepalive" ]]; then
        # The VarLive sample uses the standard library's uintptrkeepalive directive.
        test_gcflags+=(-gcflags command-line-arguments="$GCFLAGS -std")
    fi
    echo "Building tests $T"
    env -u GOROOT GOEXPERIMENT=tangollvm "$GO_BIN" build -o "$TBIN_PATH" "${test_gcflags[@]}" "$DIR/$T.go"
    if (( $? != 0 )); then
        echo "Failed to build test $T"
        RESULTS+=("$T ...FAILED to build")
        FAILED_TO_RUN_TESTS+=("$T")
        continue
    fi

    if [[ $T == unsafeconvert_keepalive || $T == opendefergc ]]; then
        GODEBUG=gccheckmark=1 GOGC=1 "$TBIN_PATH"
    else
        "$TBIN_PATH"
    fi
    if (( $? != 0 )); then
        echo "Failed to run test $T"
        RESULTS+=("$T ...FAILED")
        FAILED_TO_RUN_TESTS+=("$T")
        continue
    fi

    # // remove bin
    rm -f "$TBIN_PATH"

    RESULTS+=("$T ...passed")
done

# Tests kept in the upstream test directory use the same TangoLLVM flags but
# remain separate from their upstream counterparts.
for T in "${ROOTTESTS[@]}"
do
    TBIN_PATH=$test_cache/bin/$T
    echo "Building root test $T"
    if ! env -u GOROOT GOEXPERIMENT=tangollvm "$GO_BIN" build -o "$TBIN_PATH" -gcflags all="$GCFLAGS" "$DIR/../$T.go"; then
        RESULTS+=("$T ...FAILED to build")
        FAILED_TO_RUN_TESTS+=("$T")
    elif ! "$TBIN_PATH"; then
        RESULTS+=("$T ...FAILED")
        FAILED_TO_RUN_TESTS+=("$T")
    else
        RESULTS+=("$T ...passed")
    fi
done

# Directory tests must compile all Go/assembly files in their own package.
for T in "${DIRTESTS[@]}"
do
    TBIN_PATH=$test_cache/bin/$T
    echo "Building directory test $T"
    if ! (
        cd "$DIR/$T" || exit 1
        if [[ -f go.mod ]]; then
            env -u GOROOT GOEXPERIMENT=tangollvm "$GO_BIN" build -o "$TBIN_PATH" -gcflags all="$GCFLAGS" .
        else
            env -u GOROOT GO111MODULE=off GOEXPERIMENT=tangollvm "$GO_BIN" build -o "$TBIN_PATH" -gcflags all="$GCFLAGS" .
        fi
    ); then
        RESULTS+=("$T ...FAILED to build")
        FAILED_TO_RUN_TESTS+=("$T")
    elif ! "$TBIN_PATH"; then
        RESULTS+=("$T ...FAILED")
        FAILED_TO_RUN_TESTS+=("$T")
    else
        RESULTS+=("$T ...passed")
    fi
done


# Build the FuncPCABI0 fixture as a standard-library package so it can import
# internal/abi. Overlay the files onto the otherwise empty src/internal.
fz3temp=$(mktemp -d "${TMPDIR:-/tmp}/tangollvm-fz3.XXXXXX") || exit 1
goroot=$(env -u GOROOT "$GO_BIN" env GOROOT)
printf '{"Replace":{"%s/src/internal/fz3main.go":"%s/fz3abi/main.go","%s/src/internal/fz3identity_amd64.s":"%s/fz3abi/identity_amd64.s"}}\n' \
    "$goroot" "$DIR" "$goroot" "$DIR" > "$fz3temp/overlay.json"
# FuncPC's uintptr conversion also needs the existing conversion opt-in.
fz3flags=${GCFLAGS/-beast=/-beast=opconvert=1,}
if env -u GOROOT GOEXPERIMENT=tangollvm "$GO_BIN" build -overlay "$fz3temp/overlay.json" \
    -o "$fz3temp/fz3abi" -gcflags all="$GCFLAGS" -gcflags internal="$fz3flags -+" -ldflags=-checklinkname=0 internal \
    && "$fz3temp/fz3abi"; then
    RESULTS+=("fz3abi ...passed")
else
    RESULTS+=("fz3abi ...FAILED")
    FAILED_TO_RUN_TESTS+=("fz3abi")
fi
rm -rf "$fz3temp"

echo "============="
for R in "${RESULTS[@]}"
do
    echo $R
done
echo "============="


if [ ${#FAILED_TO_RUN_TESTS[@]} -eq 0 ]; then
    echo "----------> ALL TESTS GOT PASSED" >> failures.txt
    exit 0
fi

if [ ${#FAILED_TO_RUN_TESTS[@]} -ne 0 ]; then
    echo "----------> Failed to run tests:" >> failures.txt
    for R in "${FAILED_TO_RUN_TESTS[@]}"
    do
        echo $R >> failures.txt
    done
fi

exit 1
