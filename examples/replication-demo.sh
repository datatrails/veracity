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


FULL_REPLICA=false

# To give a feel for the replication process, we will watch for changes in a
# single tenant's log. This is not necessary for normal replication.
MONITOR_CHANGES_FOR_TENANT="tenant/6ea5cd00-c711-3649-6914-7b125928bbb4"

REPLICADIR=merklelogs

SHASUM_BIN=${SHASUM_BIN:-shasum}

# interval between replication attempts in seconds, in real use, this would be daily or longer
REPLICATION_INTERVAL=3

usage() {
    cat >&2 <<END

usage: $SCRIPTNAME 

    -f              set to replicate all tenants
    -d              veracity replicate-logs --replicadir value, default: $REPLICADIR
    -m              when replicating all tenants using -f, use this option to
                    explicitly watch for changes in a single tenant. defaults to the public tenant:
    -s              interval between replication attempts in seconds, default $REPLICATION_INTERVAL
END
    exit 1
}

while getopts "d:fo:s:t:" o; do
    case "${o}" in
        d)  REPLICADIR=$OPTARG
            ;;
        f)  FULL_REPLICA=true
            ;;
        m)  MONITOR_CHANGES_FOR_TENANT=$OPTARG
            ;;
        s)  REPLICATION_INTERVAL=$OPTARG
            ;;
        *)  usage
            ;;
    esac
done
shift $((OPTIND-1))

[ $# -gt 0 ] && echo "unexpected arguments: $@" && usage


run() {
    # default to replicating a single tenant and monitoring that same tenant for
    # changes.
    local tenants_to_replicate=$MONITOR_CHANGES_FOR_TENANT

    # If the user has asked to replicate all tenants, we clear the option that
    # specifies an explicit set of tenants to replicate.
    if $FULL_REPLICA; then
        # The default, when no tenants are specified to replicate-logs, is to replicate all tenants
        tenants_to_replicate=""
    fi

    $VERACITY_BIN --data-url $DATATRAILS_URL/verifiabledata \
         $tenants_to_replicate replicate-logs --progress --latest --replicadir=$REPLICADIR


    # identify the filename of the last massif replicated for the tenant
    local last_massif=$(ls $REPLICADIR/$MONITOR_CHANGES_FOR_TENANT/0/massifs/*.log | sort -n | tail -n 1)
    echo "last_massif: $last_massif"

    # take its hash so we can tell if it changed
    local sum_last=$($SHASUM_BIN $last_massif | awk '{print $1}')

    while true; do

        $VERACITY_BIN --data-url $DATATRAILS_URL/verifiabledata \
            $tenants_to_replicate replicate-logs  --progress --latest --replicadir=$REPLICADIR

        # This handles a case that is only significant to the way this script
        # reports. In normal use there is no need to do this.
        local new_last_massif=$(ls $REPLICADIR/$MONITOR_CHANGES_FOR_TENANT/0/massifs/*.log | sort -n | tail -n 1)
        if [ "$last_massif" != "$new_last_massif" ]; then
            last_massif=$new_last_massif
        fi

        local sum_cur=$($SHASUM_BIN $last_massif | awk '{print $1}')
        if [ "$sum_last" != "$sum_cur" ]; then
            echo "The log grew for tenant $MONITOR_CHANGES_FOR_TENANT, old hash: $sum_last, new hash: $sum_cur"
            sum_last=$sum_cur
        fi
        echo "Sleeping for $REPLICATION_INTERVAL seconds (Use Ctrl-C to exit)"
        sleep $REPLICATION_INTERVAL
    done;
}

run
