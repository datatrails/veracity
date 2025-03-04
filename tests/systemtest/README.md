## *-interactive.sh tests

There are significant areas of replication that can only be tested
interactively.  Firstly because the log needs to grow to cause it to change.
And then, secondly, because azurite doesn't support tags we can't use
integration tests for this.

In the absence of a better solution we have added the ability to run a single
interactive test which waits for some condition that is dependent on the person
running the script to interact with datatrails.

The existing -interactive.sh tests simply wait for the log replica for  the public tenant to grow.

1. cd tests/systemtest
1. shunit/shunit2 replicate-logs-latest-interactive.sh
1. go do app.datatrails.ai and record a *public* event
1. the test should pass in a few seconds after this at most.