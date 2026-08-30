#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
data="$root/data"
base="https://storage.googleapis.com/cvdf-datasets/mnist"
mkdir -p "$data"

for name in train-images-idx3-ubyte train-labels-idx1-ubyte t10k-images-idx3-ubyte t10k-labels-idx1-ubyte
do
    if [ ! -f "$data/$name" ]; then
        curl -fL "$base/$name.gz" | gzip -dc > "$data/$name.part"
        mv "$data/$name.part" "$data/$name"
    fi
done
