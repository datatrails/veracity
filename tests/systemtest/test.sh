#! /bin/bash

VERACITY_INSTALL=${VERACITY_INSTALL:-../../veracity}
DATATRAILS_URL=${DATATRAILS_URL:-https://app.datatrails.ai}
PUBLIC_ASSET_ID=${PUBLIC_ASSET_ID:-publicassets/87dd2e5a-42b4-49a5-8693-97f40a5af7f8}
PUBLIC_EVENT_ID=${PUBLIC_EVENT_ID:-publicassets/87dd2e5a-42b4-49a5-8693-97f40a5af7f8/events/a022f458-8e55-4d63-a200-4172a42fc2aa}

PROD_PUBLIC_TENANT_ID=${PROD_PUBLIC_TENANT_ID:-tenant/6ea5cd00-c711-3649-6914-7b125928bbb4}
SOAK_PUBLIC_TENANT_ID=${SOAK_PUBLIC_TENANT_ID:-tenant/2280c2c6-21c9-67b2-1e16-1c008a709ff0}

PROD_LOG_URL=${PROD_LOG_URL:-${DATATRAILS_URL}/verifiabledata/merklelogs/v1/mmrs/${PROD_PUBLIC_TENANT_ID}/0/massifs/0000000000000000.log}
SOAK_LOG_URL=${SOAK_LOG_URL:-https://app.soak.stage.datatrails.ai/verifiabledata/merklelogs/v1/mmrs/${SOAK_PUBLIC_TENANT_ID}/0/massifs/0000000000000000.log}
TEST_TMPDIR=${TEST_TMPDIR:-${SHUNIT_TMPDIR}}
EMPTY_DIR=$TEST_TMPDIR/empty
PROD_DIR=$TEST_TMPDIR/prod
SOAK_DIR=$TEST_TMPDIR/soak
DUP_DIR=$TEST_TMPDIR/duplicate-massifs
PROD_LOCAL_BLOB_FILE="$PROD_DIR/mmr.log"
SOAK_LOCAL_BLOB_FILE="$SOAK_DIR/soak-mmr.log"
INVALID_BLOB_FILE="$TEST_TMPDIR/invalid.log"

oneTimeSetUp() {
    mkdir -p $EMPTY_DIR
    mkdir -p $PROD_DIR
    mkdir -p $SOAK_DIR
    mkdir -p $DUP_DIR
    curl -s -H "x-ms-blob-type: BlockBlob" -H "x-ms-version: 2019-12-12" $PROD_LOG_URL -o $PROD_LOCAL_BLOB_FILE
    curl -s -H "x-ms-blob-type: BlockBlob" -H "x-ms-version: 2019-12-12" $SOAK_LOG_URL -o $SOAK_LOCAL_BLOB_FILE
    touch $INVALID_BLOB_FILE

    # Duplicate the prod and soak massif files in a single directory. The
    # replication should refuse to work with a directory that has multiple
    # massif files for the same massif index.
    cp $PROD_LOCAL_BLOB_FILE $DUP_DIR/prod-mmr.log
    cp $SOAK_LOCAL_BLOB_FILE $DUP_DIR/soak-mmr.log

    assertTrue "prod MMR blob file should be present" "[ -r $PROD_LOCAL_BLOB_FILE ]"
    assertTrue "soak MMR blob file should be present" "[ -r $SOAK_LOCAL_BLOB_FILE ]"
    assertTrue "invalid MMR blob file should be present" "[ -r $INVALID_BLOB_FILE ]"
}

assertStringMatch() {
    local message="$1"
    local expected="$2"
    local actual="$3"

    # Normalize by converting all spaces to a single space, removing leading/trailing spaces and punctuation.
    expected=$(echo "$expected" | sed -e 's/[[:space:]]\+/ /g' -e 's/^[[:space:]]*//;s/[[:space:]]*[[:punct:]]*$//')
    actual=$(echo "$actual" | sed -e 's/[[:space:]]\+/ /g' -e 's/^[[:space:]]*//;s/[[:space:]]*[[:punct:]]*$//')

    echo "Expected (hex):" && echo "$expected" | hexdump -C
    echo "Actual (hex):" && echo "$actual" | hexdump -C


    assertEquals "$message" "$expected" "$actual"
}

testVeracityVersion() {
    local output
    output=$($VERACITY_INSTALL --version)
    assertEquals "veracity --version should return a 0 exit code" 0 $?

    echo "$output" | grep -qE '^v[0-9]+\.[0-9]+\.[0-9]+'
    assertTrue "The output should start with a semantic version string" $?
}

testVeracityWatchPublicFindsActivity() {
    local output
    output=$($VERACITY_INSTALL --data-url $DATATRAILS_URL/verifiabledata --tenant=$PROD_PUBLIC_TENANT_ID watch --horizon 10000h)
    assertEquals "watch-public should return a 0 exit code" 0 $?
    assertContains "watch-public should find activity" "$output" "$PROD_PUBLIC_TENANT_ID"
}

testVeracityWatchLatestFindsActivity() {
    local output
    output=$($VERACITY_INSTALL --data-url $DATATRAILS_URL/verifiabledata --tenant=$PROD_PUBLIC_TENANT_ID watch --latest)
    assertEquals "watch-public --latest should return a 0 exit code" 0 $?
    assertContains "watch-public --latest should find activity" "$output" "$PROD_PUBLIC_TENANT_ID"
}

testVeracityReplicateLogsPublicTenantWatchPipe() {
    local output
    local replicadir=$TEST_TMPDIR/merklelogs

    rm -rf $replicadir
    output=$($VERACITY_INSTALL --data-url $DATATRAILS_URL/verifiabledata \
        --tenant=$PROD_PUBLIC_TENANT_ID watch --horizon 10000h \
        | $VERACITY_INSTALL --data-url $DATATRAILS_URL/verifiabledata --tenant=$PROD_PUBLIC_TENANT_ID replicate-logs --ancestors=0 --replicadir=$replicadir)
    assertEquals "watch-public should return a 0 exit code" 0 $?

    COUNT=$(find $replicadir -type f | wc -l | tr -d ' ')
    assertEquals "should replicate one massif and one seal" "2" "$COUNT"
}

testVeracityReplicateLogsPublicTenantWatchLatestFlag() {
    local output
    local replicadir=$TEST_TMPDIR/merklelogs

    rm -rf $replicadir
    output=$($VERACITY_INSTALL --data-url $DATATRAILS_URL/verifiabledata --tenant=$PROD_PUBLIC_TENANT_ID replicate-logs --latest --ancestors=0 --replicadir=$replicadir)
    assertEquals "replicate-logs --latest should return a 0 exit code" 0 $?

    COUNT=$(find $replicadir -type f | wc -l | tr -d ' ')
    assertEquals "should replicate one massif and one seal" "2" "$COUNT"
}

testVerifySingleEvent() {
    # Check if the response status code is 200
    local response
    response=$(curl -sL -w "%{http_code}" $DATATRAILS_URL/archivist/v2/$PUBLIC_EVENT_ID -o /dev/null)
    assertEquals 200 "$response"
    # Verify the event and check if the exit code is 0
    curl -sL $DATATRAILS_URL/archivist/v2/$PUBLIC_EVENT_ID | $VERACITY_INSTALL --data-url $DATATRAILS_URL/verifiabledata --tenant=$PROD_PUBLIC_TENANT_ID verify-included
    assertEquals "Verifying a valid single event should result in a 0 exit code" 0 $?
}

testVerifyListEvents() {
    # Check if the response status code is 200
    local response
    response=$(curl -sL -w "%{http_code}" $DATATRAILS_URL/archivist/v2/$PUBLIC_ASSET_ID/events -o /dev/null)
    assertEquals 200 "$response"
    # Verify the events on the asset and check if the exit code is 0
    curl -sL $DATATRAILS_URL/archivist/v2/$PUBLIC_ASSET_ID/events | $VERACITY_INSTALL --data-url $DATATRAILS_URL/verifiabledata --tenant=$PROD_PUBLIC_TENANT_ID verify-included
    assertEquals "Verifying events on a vaid asset should result in a 0 exit code" 0 $?
}

testVerifySingleEventWithLocalMassifCopy() {
    # Verify the event and check if the exit code is 0
    curl -sL $DATATRAILS_URL/archivist/v2/$PUBLIC_EVENT_ID | $VERACITY_INSTALL --data-local $PROD_LOCAL_BLOB_FILE --tenant=$PROD_PUBLIC_TENANT_ID verify-included
    assertEquals "verifying valid events with a local copy of the massif should result in a 0 exit code" 0 $?
}

testFindTrieEntrySingleEvent() {
    # Verify the trie key for the known event is on the log at the correct position.
    PUBLIC_EVENT_PERMISSIONED_ID=${PUBLIC_EVENT_ID#"public"}
    output=$(VERACITY_IKWID=true $VERACITY_INSTALL find-trie-entries --log-tenant $PROD_PUBLIC_TENANT_ID --app-id $PUBLIC_EVENT_PERMISSIONED_ID)
    assertEquals "verifying finding the trie entry of a known public prod event from the datatrails log should match mmr index 663" "matches: [663]" "$output"
}

testFindTrieEntrySingleEventWithLocalMassifCopy() {
    # Verify the trie key for the known event is on the log at the correct position for a local log.
    PUBLIC_EVENT_PERMISSIONED_ID=${PUBLIC_EVENT_ID#"public"}
    output=$(VERACITY_IKWID=true $VERACITY_INSTALL --data-local $PROD_LOCAL_BLOB_FILE find-trie-entries --log-tenant $PROD_PUBLIC_TENANT_ID --app-id $PUBLIC_EVENT_PERMISSIONED_ID)
    assertEquals "verifying finding the trie entry of a known public prod event from a local log should match mmr index 663" "matches: [663]" "$output"
}

testFindMMREntrySingleEvent() {
    # Verify the mmr entry for the known event is on the log at the correct position.
    output=$(curl -sL $DATATRAILS_URL/archivist/v2/$PUBLIC_EVENT_ID | VERACITY_IKWID=true $VERACITY_INSTALL find-mmr-entries --log-tenant $PROD_PUBLIC_TENANT_ID)
    assertEquals "verifying finding the mmr entry of a known public prod event from the datatrails log should match mmr index 663" "matches: [663]" "$output"
}

testFindMMREntrySingleEventWithLocalMassifCopy() {
    # Verify the mmr entry for the known event is on the log at the correct position.
    output=$(curl -sL $DATATRAILS_URL/archivist/v2/$PUBLIC_EVENT_ID | VERACITY_IKWID=true $VERACITY_INSTALL --data-local $PROD_LOCAL_BLOB_FILE find-mmr-entries --log-tenant $PROD_PUBLIC_TENANT_ID)
    assertEquals "verifying finding the mmr entry of a known public prod event from a local log should match mmr index 663" "matches: [663]" "$output"
}

testReplicateErrorForLogShorterThanSeal() {

    local output
    local other_tenant
    local tampered_log_url
    local tampered_seal_url
    local replicadir=$TEST_TMPDIR/merklelogs

    # Note: this tenant belongs to Joe Gough and he has promised never to fill the first massif
    other_tenant=tenant/97e90a09-8c56-40df-a4de-42fde462ef6f

    rm -rf $replicadir
    # first get the prod public tenant replicated for massif 0. NOTE: this is a full massif
    output=$($VERACITY_INSTALL --data-url $DATATRAILS_URL/verifiabledata --tenant=$PROD_PUBLIC_TENANT_ID replicate-logs --massif 0 --replicadir=$replicadir)
    assertEquals "0: should return a 0 exit code" 0 $?

    COUNT=$(find $replicadir -type f | wc -l | tr -d ' ')
    assertEquals "should replicate one massif and one seal" "2" "$COUNT"

    # now get a different prod tenant log and seal. NOTE the log is partially full for this tenant
    tampered_log_url=${tampered_log_url:-${DATATRAILS_URL}/verifiabledata/merklelogs/v1/mmrs/${other_tenant}/0/massifs/0000000000000000.log}
    tampered_seal_url=${tampered_seal_url:-${DATATRAILS_URL}/verifiabledata/merklelogs/v1/mmrs/${other_tenant}/0/massifseals/0000000000000000.sth}
    curl -s -H "x-ms-blob-type: BlockBlob" -H "x-ms-version: 2019-12-12" $tampered_log_url -o tampered.log

    # copy over the different (shorter) tenant log for massif 0
    cp tampered.log $replicadir/$PROD_PUBLIC_TENANT_ID/0/massifs/0000000000000000.log

    # attempt to replicate the logs again, the local log data is for the wrong
    # tenant and is *less* than the seal expects, but the local seal is correct
    # for the replaced data and the remote seal
    output=$($VERACITY_INSTALL --data-url $DATATRAILS_URL/verifiabledata --tenant=$PROD_PUBLIC_TENANT_ID replicate-logs --replicadir=$replicadir)
    assertEquals "1: a tampered log should exit 1" 1 $?
    assertContains "$output"  "error: There is insufficient data in the massif context to generate a consistency proof against the provided state"
}

# test veracity can't extend the replica of the wrong tenant
#
# When extending a local replica, if the local tenant log data is from a tenant
# other than the requested remote, replication should fail due to consistency
# checks. This is essentially equivalent to a tamper attempt.
#
# There are two cases that are important:
#   1. The higest indexed local massif is incomplete and so the remote massif is used to extend it.
#   2. The highest indexed local massif is complete and so the remote massif is copied, leaving the original unchanged.
#
# This test suite covers only the second case. Thee first case can only be
# tested with synthesized data, or interaction with a live system, and so is
# easier to cover in the integration tests. However, the same checks are
# excercised in both cases and so this test gives a lot of confidence both
# situations are sound.
#
# Note that the --ancestor flag can be used to limit how many massifs are
# replicated. This can cause the replica to "start again" because the replica is
# so far behind that the --ancestor limit forces a gap. In this case consistency
# of the remote is not checked against the local massif, and in that case the
# replication would succeded, the local replica of the foregn tenant would not
# be updated. And the replica would be left with massifs from multiple tenants.
testReplicateErrorForMixedTenants() {

    local output
    local other_tenant
    local tampered_log_url
    local tampered_seal_url
    local replicadir=$TEST_TMPDIR/merklelogs

    # Note: this tenant is known to have > 1 massif at the time of writing and logs don't get shorter
    other_tenant=tenant/b197ba3c-44fe-4b1a-bbe8-bd9674b2bd17

    rm -rf $replicadir
    # first get the prod public tenant replicated for massif 0. NOTE: this is a full massif
    output=$($VERACITY_INSTALL --data-url $DATATRAILS_URL/verifiabledata --tenant=$PROD_PUBLIC_TENANT_ID replicate-logs --massif 0 --replicadir=$replicadir)
    assertEquals "should return a 0 exit code" 0 $?

    COUNT=$(find $replicadir -type f | wc -l | tr -d ' ')
    assertEquals "should replicate one massif and one seal" "2" "$COUNT"

    # now get a different prod tenant log and seal. NOTE the log is partially full for this tenant
    tampered_log_url=${tampered_log_url:-${DATATRAILS_URL}/verifiabledata/merklelogs/v1/mmrs/${other_tenant}/0/massifs/0000000000000000.log}
    tampered_seal_url=${tampered_seal_url:-${DATATRAILS_URL}/verifiabledata/merklelogs/v1/mmrs/${other_tenant}/0/massifseals/0000000000000000.sth}
    curl -s -H "x-ms-blob-type: BlockBlob" -H "x-ms-version: 2019-12-12" $tampered_log_url -o tampered.log
    curl -s -H "x-ms-blob-type: BlockBlob" -H "x-ms-version: 2019-12-12" $tampered_seal_url -o tampered.sth

    # copy over the different tenant log for massif 0
    cp tampered.log $replicadir/$PROD_PUBLIC_TENANT_ID/0/massifs/0000000000000000.log

    # attempt to replicate the logs again, the local log data is for the wrong tenant but the local seal is correct for the replaced data and the remote seal
    output=$($VERACITY_INSTALL --data-url $DATATRAILS_URL/verifiabledata --tenant=$PROD_PUBLIC_TENANT_ID replicate-logs --replicadir=$replicadir)
    assertEquals "1: extending an inconsistent replica should exit 1" 1 $?
    assertContains "$output"  "error: the seal signature verification failed: failed to verify seal for massif 0"

    # now add in the seal from the other log, so that the local log and seal are consistent and locally verifiable.
    cp tampered.sth $replicadir/$PROD_PUBLIC_TENANT_ID/0/massifseals/0000000000000000.sth

    # attempt to replicate the logs again
    output=$($VERACITY_INSTALL --data-url $DATATRAILS_URL/verifiabledata --tenant=$PROD_PUBLIC_TENANT_ID replicate-logs --latest --replicadir=$replicadir)
    assertEquals "2: extending an inconsistent replica should exit 1" 1 $?
    assertContains "$output"  "error: consistency check failed: the accumulator produced for the trusted base state doesn't match the root produced for the seal state fetched from the log"
}

# test veracity can't update the replica for a tenant whos log has been tampered with
#
# This test repeats testReplicateErrorForMixedTenants, but does so using the
# combination of watch | replicate-logs which permits finer control over the
# replica
testReplicateLocalErrorForTamperedPartialToFullMassif() {

    local output
    local other_tenant
    local tampered_log_url
    local tampered_seal_url

    local replicadir=$TEST_TMPDIR/merklelogs

    # Note: this tenant is known to have > 1 massif at the time of writing and logs don't get shorter
    other_tenant=tenant/b197ba3c-44fe-4b1a-bbe8-bd9674b2bd17

    # first get the prod tenant replicated for massif 0. NOTE: this is a partially full massif
    rm -rf $replicadir
    output=$($VERACITY_INSTALL --data-url $DATATRAILS_URL/verifiabledata \
        --tenant=$other_tenant watch --horizon 10000h \
        | $VERACITY_INSTALL --data-url $DATATRAILS_URL/verifiabledata --tenant=$other_tenant replicate-logs --ancestors=0 --massif 0 --replicadir=$replicadir)
    assertEquals "watch-public should return a 0 exit code" 0 $?

    COUNT=$(find $replicadir -type f | wc -l | tr -d ' ')
    assertEquals "should replicate one massif and one seal" "2" "$COUNT"

    # now get a different prod public tenant log and seal. NOTE: this is a full massif
    tampered_log_url=${tampered_log_url:-${DATATRAILS_URL}/verifiabledata/merklelogs/v1/mmrs/${PROD_PUBLIC_TENANT_ID}/0/massifs/0000000000000000.log}
    tampered_seal_url=${tampered_seal_url:-${DATATRAILS_URL}/verifiabledata/merklelogs/v1/mmrs/${PROD_PUBLIC_TENANT_ID}/0/massifseals/0000000000000000.sth}
    curl -s -H "x-ms-blob-type: BlockBlob" -H "x-ms-version: 2019-12-12" $tampered_log_url -o tampered.log
    curl -s -H "x-ms-blob-type: BlockBlob" -H "x-ms-version: 2019-12-12" $tampered_seal_url -o tampered.sth

    # copy over the different tenant log for massif 0
    cp tampered.log $replicadir/$other_tenant/0/massifs/0000000000000000.log

    # attempt to replicate the logs again
    output=$($VERACITY_INSTALL --data-url $DATATRAILS_URL/verifiabledata \
        --tenant=$other_tenant watch --horizon 10000h \
        | $VERACITY_INSTALL --data-url $DATATRAILS_URL/verifiabledata --tenant=$other_tenant replicate-logs --replicadir=$replicadir)
    assertEquals "extending an inconsistent replica should exit 1" 1 $?
    assertContains "$output"  "error: the seal signature verification failed: failed to verify seal for massif 0"

    # now attempt to change the seal to the tampered log seal
    cp tampered.sth $replicadir/$other_tenant/0/massifseals/0000000000000000.sth

    # attempt to replicate the logs again
    output=$($VERACITY_INSTALL --data-url $DATATRAILS_URL/verifiabledata \
        --tenant=$other_tenant watch --horizon 10000h \
        | $VERACITY_INSTALL --data-url $DATATRAILS_URL/verifiabledata --tenant=$other_tenant replicate-logs --replicadir=$replicadir)
    assertEquals "extending an inconsistent replica should exit 1" 1 $?
    assertContains "$output"  "error: consistency check failed"
}

testVerboseOutput() {
    local expected_output="verifying events dir: defaulting to the standard container merklelogs verifying for tenant: $PROD_PUBLIC_TENANT_ID verifying: 663 334 018fa97ef269039b00 publicassets/87dd2e5a-42b4-49a5-8693-97f40a5af7f8/events/a022f458-8e55-4d63-a200-4172a42fc2aa OK|663 334|[aea799fb2a8c4bbb6eda1dd2c1e69f8807b9b06deeaf51b9e0287492cefd8e4c, 9f0183c7f79fd81966e104520af0f90c8447f1a73d4e38e7f2f23a0602ceb617, da21cb383d63896a9811f06ebd2094921581d8eb72f7fbef566b730958dc35f1, 51ea08fd02da3633b72ef0b09d8ba4209db1092d22367ef565f35e0afd4b0fc3, 185a9d55cf507ef85bd264f4db7228e225032c48da689aa8597e11059f45ab30, bab40107f7d7bebfe30c9cea4772f9eb3115cae1f801adab318f90fcdc204bdc, 94ca607094ead6fcd23f52851c8cdd8c6f0e2abde20dca19ba5abc8aff70d0d1, ba6d0fd8922342aafbba6073c5510103b077a7de9cb2d72fb652510110250f9e, 7fafc7edc434225afffc19b0582efa2a71b06a2d035358356df0a52d2256c235, 18c9b525a75ff8386f108abed53e01f79173892bb7fe90805f749d3d3af09d28] verifying: 916 461 019007e7960d052e00 publicassets/87dd2e5a-42b4-49a5-8693-97f40a5af7f8/events/999773ed-cc92-4d9c-863f-b418418705ea OK|916 461|[25ee5db5cce059f89372dd3a54bfa6fd9f77d8a09eef36a88e2cba12631eaef6, df700cc8323dcece5185b4cdd769854369c59d9a38b364fabaebe3ad83aa2693, 1dd1250b52ed3f0a408f6928182bec55ddb2b5648c834cc1e104fe2029ec22e3, 292ce1ef003fb25f3bbdb4de5d9af91cdbf85185224f560d351ed2558723b08e, 118cbc9b298a5442177728c707dea6adf1a65274cf0a1e4ac09aa22dd38ebdb0, 27b3d13f8faf19ebaa3525c8b61825f25b772de1121d1e51f5f3d278b6ed00db, 2d7a6a491d378f5c4c97de2e2ab36bc6f8e6ec80ecd0b61f263ffcc754f10576, 302b47f6a440c664f406fb2c13996d46804983c4bab0fe978e8b5f3a4db65f78, 7fafc7edc434225afffc19b0582efa2a71b06a2d035358356df0a52d2256c235, 18c9b525a75ff8386f108abed53e01f79173892bb7fe90805f749d3d3af09d28]"
    local output

    # Verify the events on the asset and check if the exit code is 0
    output=$(curl -sL $DATATRAILS_URL/archivist/v2/$PUBLIC_ASSET_ID/events | $VERACITY_INSTALL --data-url $DATATRAILS_URL/verifiabledata --tenant=$PROD_PUBLIC_TENANT_ID --loglevel=verbose verify-included 2>&1)
    assertEquals "Verifying events on a vaid asset should result in a 0 exit code" 0 $?

    # check that the output contains the expected string
    assertContains "Verifying verbose output matches" "$expected_output" "$output"
}

testHelpOutputNoArgs() {
    local output

    output=$($VERACITY_INSTALL 2>&1)
    assertEquals "Calling veracity with no args should return a help message and a zero exit code" 0 $?
    assertNotNull "help message should be present" "$output"
}

testValidEventNotinMassif() {
    local expected_message="error: the entry is not in the log. for tenant $PROD_PUBLIC_TENANT_ID"
    local output

    output=$(curl -sL $DATATRAILS_URL/archivist/v2/$PUBLIC_EVENT_ID | $VERACITY_INSTALL --data-local $SOAK_LOCAL_BLOB_FILE --tenant=$PROD_PUBLIC_TENANT_ID verify-included 2>&1)
    assertEquals "verifying an event not in the massif should result in an error" 1 $?
    assertStringMatch "Error should have the correct error message" "$expected_message" "$output"
}

testNon200Response() {
    local invalid_event_ID=publicassets/87dd2e5a-42b4-49a5-8693-97f40a5af7f8/events/a022f458-8e55-4d63-a200-4172a42fc2ab
    local output

    output=$(curl -sL $DATATRAILS_URL/archivist/v2/$invalid_event_ID | $VERACITY_INSTALL --data-local $PROD_LOCAL_BLOB_FILE --tenant=$PROD_PUBLIC_TENANT_ID verify-included 2>&1)
    assertEquals "a non 200 response being piped in should result in a non 0 exit code" 1 $?
    assertNotNull "Error message should be present" "$output"
}

testMissingMassifFile() {
    local expected_message="error: the entry is not in the log. for tenant $PROD_PUBLIC_TENANT_ID"
    local output

    output=$(curl -sL $DATATRAILS_URL/archivist/v2/$PUBLIC_EVENT_ID | $VERACITY_INSTALL --data-local $EMPTY_DIR --tenant=$PROD_PUBLIC_TENANT_ID verify-included 2>&1)
    assertEquals "verifying an event not in the massif should result in an error" 1 $?
    assertContains "$output" "a log file corresponding to the massif index was not found"
}

testNotBlobFile() {
    local expected_message="error: the entry is not in the log. for tenant $PROD_PUBLIC_TENANT_ID"
    local output


    output=$(curl -sL $DATATRAILS_URL/archivist/v2/$PUBLIC_EVENT_ID | $VERACITY_INSTALL --data-local $INVALID_BLOB_FILE --tenant=$PROD_PUBLIC_TENANT_ID verify-included 2>&1)
    assertEquals "verifying an event not in the massif should result in an error" 1 $?
    assertContains "$output" "a massif file header was to short or badly formed"
}

testInvalidBlobUrl() {
    local expected_message="error: no json given"
    local invalid_domain="https://app.datatrails.com"
    local invalid_url="$invalid_domain/verifiabledata"
    local output
    output=$(curl -sL $invalid_domain/archivist/v2/$PUBLIC_EVENT_ID | $VERACITY_INSTALL --data-url $invalid_url --tenant=$PROD_PUBLIC_TENANT_ID verify-included 2>&1)

    assertEquals "verifying an event not in the massif should result in an error" 1 $?
    assertStringMatch "Error should have the correct error message" "$expected_message" "$output"
}
