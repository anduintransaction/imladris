#!/usr/bin/env bash

RELEASE=0.0.6
dist=dist
bin=anduin-minikube-deploy

function build {
    GOOS=$1 GOARCH=$2 go build -o $bin
    package=$bin-$RELEASE-$1-$2.tar.gz
    tar cvzf $package $bin
    mv $package $dist
    rm $bin
}

mkdir -p $dist
build darwin amd64
build linux amd64
