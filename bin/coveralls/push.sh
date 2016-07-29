#!/bin/sh

gocov convert *.cov > profile.cov.json
goveralls -gocovdata profile.cov.json -repotoken $COVERALLS_TOKEN -service travis-ci
