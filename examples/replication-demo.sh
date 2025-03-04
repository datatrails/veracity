#!/usr/bin/env bash

# This script is intended as a minimal and simple to follow demonstration of
# replication using veracity to replicate DataTrails transparency logs.

set -o errexit
set -o nounset
set -o pipefail

SCRIPTNAME=$(basename $0)

DATATRAILS_URL=${DATATRAILS_URL:-https://app.datatrails.ai}

# For development and testing setting this to "go run cmd/veracity/main.go" is useful.
VERACITY_BIN=${VERACITY_BIN:-veracity}

# Default to replicating just the public tenant. This will cause the script to
# report an update for creation of a public event by any tenant.
# use `-t ""` to replicate all tenants
REPLICATE_TENANT_OPTION="--tenant tenant/6ea5cd00-c711-3649-6914-7b125928bbb4"

# The tenant to watch for changes. We default to the public tenant so that a
# public event registered in any tenant will be spotted by this script.
MONITOR_TENANT="tenant/6ea5cd00-c711-3649-6914-7b125928bbb4"

REPLICADIR=merklelogs

SHASUM_BIN=${SHASUM_BIN:-shasum}

# interval between replication attempts in seconds, in real use, this would be daily or longer
REPLICATION_INTERVAL=3

usage() {
    cat >&2 <<END

usage: $SCRIPTNAME 

    -d              veracity replicate-logs --replicadir value, default: $REPLICADIR
    -o              --tenant option, default: $REPLICATE_TENANT_OPTION
    -s              interval between replication attempts in seconds, default $REPLICATION_INTERVAL
    -t              tenant, tenant to watch for changes default: $MONITOR_TENANT
END
    exit 1
}

while getopts "d:o:s:t:" o; do
    case "${o}" in
        d)  REPLICADIR=$OPTARG
            ;;
        o)  REPLICATE_TENANT_OPTION=$OPTARG
            ;;
        s)  REPLICATION_INTERVAL=$OPTARG
            ;;
        t)  MONITOR_TENANT=$OPTARG
            ;;
        *)  usage
            ;;
    esac
done
shift $((OPTIND-1))

[ $# -gt 0 ] && echo "unexpected arguments: $@" && usage


run() {
    $VERACITY_BIN --data-url $DATATRAILS_URL/verifiabledata \
         $REPLICATE_TENANT_OPTION replicate-logs --progress --latest --replicadir=$REPLICADIR

    # identify the filename of the last massif replicated for the tnenant
    local last_massif=$(ls $REPLICADIR/$MONITOR_TENANT/0/massifs/*.log | sort -n | tail -n 1)
    echo "last_massif: $last_massif"

    # take its hash so we can tell if it changed
    local sum_last=$($SHASUM_BIN $last_massif | awk '{print $1}')

    while true; do

        $VERACITY_BIN --data-url $DATATRAILS_URL/verifiabledata \
            $REPLICATE_TENANT_OPTION replicate-logs  --progress --latest --replicadir=$REPLICADIR

        # This handles a case that is only significant to the way this script
        # reports. In normal use there is no need to do this.
        local new_last_massif=$(ls $REPLICADIR/$MONITOR_TENANT/0/massifs/*.log | sort -n | tail -n 1)
        if [ "$last_massif" != "$new_last_massif" ]; then
            last_massif=$new_last_massif
        fi

        local sum_cur=$($SHASUM_BIN $last_massif | awk '{print $1}')
        if [ "$sum_last" != "$sum_cur" ]; then
            echo "The log grew for tenant $MONITOR_TENANT, old hash: $sum_last, new hash: $sum_cur"
            sum_cur=$sum_last
        fi
        echo "Sleeping for $REPLICATION_INTERVAL seconds (Use Ctrl-C to exit)"
        sleep $REPLICATION_INTERVAL
    done;
}

run
