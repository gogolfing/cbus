#!/bin/sh

retval=0

for package in $(find . -name "*.go" | xargs -n 1 dirname | sort -u); do
    go test -v -covermode=count -coverprofile=$(echo ${package} \
        | sed -e "s/[./]\+/_/g").cov ${package} || retval=$?;
done;

exit $retval
