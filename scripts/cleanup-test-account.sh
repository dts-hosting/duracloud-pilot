#!/bin/bash

STACK={1:-duracloud-lyrasis}

make bucket action=empty bucket=${STACK}-bucket-requested > /dev/null
make bucket action=empty bucket=${STACK}-logs > /dev/null
make bucket action=empty bucket=${STACK}-managed > /dev/null

make bucket action=empty bucket=${STACK}-pilot-ex-testing123 > /dev/null
make bucket action=delete bucket=${STACK}-pilot-ex-testing123
make bucket action=delete bucket=${STACK}-pilot-ex-testing123-replication

make bucket action=empty bucket=${STACK}-pilot-ex-testing456 > /dev/null
make bucket action=delete bucket=${STACK}-pilot-ex-testing456
make bucket action=delete bucket=${STACK}-pilot-ex-testing456-replication

make bucket action=empty bucket=${STACK}-pilot-ex-testing789-public > /dev/null
make bucket action=delete bucket=${STACK}-pilot-ex-testing789-public
make bucket action=delete bucket=${STACK}-pilot-ex-testing789-public-replication
