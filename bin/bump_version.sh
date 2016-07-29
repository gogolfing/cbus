#!/bin/sh

#ensure we are on the master branch.
branch=$(git branch | grep "*" | cut -d " " -f 2)
if [ "$branch" != "master" ]; then
    >&2 echo "you must be on master branch to run this command"
    exit 1
fi

#grab the version argument.
version="$1"

if [ -z "$version" ]; then
    >&2 echo "usage: first argument supplied must be a non-empty version"
    exit 2
fi

#set tag on new version commit.
git tag -a "$version"
did_tag=$?

if [ $did_tag -ne 0 ]; then
    exit 3
fi

#push master branch and new tag to origin.
git push origin master tag "$version"
did_push=$?

echo "\n"

if [ $did_push -eq 0 ]; then
    echo "Version bump, commit, tag, and push to origin/master successful!!"
else
    >&2 echo "failed to push new tag to origin/master"
    exit 4
fi
