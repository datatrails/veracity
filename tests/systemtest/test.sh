#! /bin/bash

VERACITY_INSTALL=${VERACITY_INSTALL:-../../veracity}
DATATRAILS_URL=${DATATRAILS_URL:-https://app.datatrails.ai}
PUBLIC_ASSET_ID=${PUBLIC_ASSET_ID:-publicassets/87dd2e5a-42b4-49a5-8693-97f40a5af7f8}
PUBLIC_EVENT_ID=${PUBLIC_EVENT_ID:-publicassets/87dd2e5a-42b4-49a5-8693-97f40a5af7f8/events/a022f458-8e55-4d63-a200-4172a42fc2aa}

PROD_PUBLIC_TENANT_ID=${PROD_PUBLIC_TENANT_ID:-tenant/6ea5cd00-c711-3649-6914-7b125928bbb4}
SOAK_PUBLIC_TENANT_ID=${SOAK_PUBLIC_TENANT_ID:-tenant/2280c2c6-21c9-67b2-1e16-1c008a709ff0}

PROD_LOG_URL=${PROD_LOG_URL:-${DATATRAILS_URL}/verifiabledata/merklelogs/v1/mmrs/${PROD_PUBLIC_TENANT_ID}/0/massifs/0000000000000000.log}
SOAK_LOG_URL=${SOAK_LOG_URL:-https://app.soak.stage.datatrails.ai/verifiabledata/merklelogs/v1/mmrs/${SOAK_PUBLIC_TENANT_ID}/0/massifs/0000000000000000.log}

PROD_LOCAL_BLOB_FILE="$SHUNIT_TMPDIR/mmr.log"
SOAK_LOCAL_BLOB_FILE="$SHUNIT_TMPDIR/soak-mmr.log"
INVALID_BLOB_FILE="$SHUNIT_TMPDIR/invalid.log"
EMPTY_DIR=$SHUNIT_TMPDIR/empty

oneTimeSetUp() {
    mkdir -p $EMPTY_DIR
    curl -s -H "x-ms-blob-type: BlockBlob" -H "x-ms-version: 2019-12-12" $PROD_LOG_URL -o $PROD_LOCAL_BLOB_FILE
    curl -s -H "x-ms-blob-type: BlockBlob" -H "x-ms-version: 2019-12-12" $SOAK_LOG_URL -o $SOAK_LOCAL_BLOB_FILE
    touch $INVALID_BLOB_FILE

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
    assertStringMatch "Error should have the correct error message" "$output" "$expected_message"
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
    assertStringMatch "Error should have the correct error message" "$output" "$expected_message"
}

testNotBlobFile() {
    local expected_message="error: the entry is not in the log. for tenant $PROD_PUBLIC_TENANT_ID"
    local output


    output=$(curl -sL $DATATRAILS_URL/archivist/v2/$PUBLIC_EVENT_ID | $VERACITY_INSTALL --data-local $INVALID_BLOB_FILE --tenant=$PROD_PUBLIC_TENANT_ID verify-included 2>&1)
    assertEquals "verifying an event not in the massif should result in an error" 1 $?
    assertStringMatch "Error should have the correct error message" "$output" "$expected_message"
}

testInvalidBlobUrl() {
    local expected_message="error: unexpected end of JSON input"
    local invalid_domain="https://app.datatrails.com"
    local invalid_url="$invalid_domain/verifiabledata"
    local output
    output=$(curl -sL $invalid_domain/archivist/v2/$PUBLIC_EVENT_ID | $VERACITY_INSTALL --data-url $invalid_url --tenant=$PROD_PUBLIC_TENANT_ID verify-included 2>&1)

    assertEquals "verifying an event not in the massif should result in an error" 1 $?
    assertStringMatch "Error should have the correct error message" "$output" "$expected_message"
}
