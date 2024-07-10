# veracity

Veracity is a command line tool providing support for inspecting DataTrails native `MERKLE_LOG` verifiable data.

Familiarity with a command line environment on your chosen platform is assumed
by this README.

A general familiarity with verifiable data structures, and in particular binary
merkle trees, would be advantageous when using `veractity` but is not required.

## Support

We provide pre-built native binaries for linux, mac, and windows. The
following architectures are supported:

| Platform      | Architecture |
| :--------     | -----------: |
| MacOS(darwin) | arm64        |
| MacOS(darwin) | x86_64       |
| Linux         | arm64        |
| Linux         | x86_64       |
| Windows       | x86_64       |
| Windows       | i386         |

The linux binaries can also be used in Windows Subsystem for Linux.

## Installation


Installation is a manual process:

1. Download the archive for your host platform
2. Extract the archive
3. Set the file permissions
4. Move the binary to a location on your PATH

For example, for the Linux or Darwin OS the following steps would be conventional

```
PLATFORM=Darwin
ARCH=arm64
VERSION=0.0.1
curl -sLO https://github.com/datatrails/veracity/releases/download/v${VERSION}/veracity_${PLATFORM}_${ARCH}.tar.gz
chmod +x ./veracity
./veracity --help
```

Set PLATFORM and ARCH according to you environment. Select the desired release
from the [releases page](https://github.com/datatrails/veracity/releases) as VERSION (Omitting the 'v').

The last step should report usage information. Usual practice is to move the
binary into a location on your $PATH. For example:

```
mkdir -p $HOME/bin
mv ./veracity $HOME/bin/
which veracity
```

The last command will echo the location of the veracity binary if $HOME/bin is
in your $PATH

# A simple first example using `nodescan`


`nodescan` is a command which searches for a leaf entry in the verifiable data by linearly
scanning the log. This is typically used in development as a diagnostic aid.
It can also be used for some audit use cases.

Find a leaf in the log by full audit. The Merkle Leaf value for any DataTrails event
can be found from its event details page in the UI. Follow the "Merkle Log Entry" link.

```
URL=https://app.datatrails.ai/verifiabledata
TENANT=tenant/7dfaa5ef-226f-4f40-90a5-c015e59998a8
LEAF=2b8ecdee967d976a31bac630036d6b183bd40913f969b47b438d4614ce7fa155

veracity --url $URL --tenant=$TENANT nodescan -v $LEAF
```

This command will report the MMR index of that leaf as `10`

The conventional way to visualise the MMR index is like this

```

     6
   /  \
  2    5     9
 /\   / \   / \  
0  1  3  4 7  8  10  MMR INDEX

0  1  2  3 5  5   6 LEAF INDEX
```

And that shows that the leaf, which has MMR index `10` is the *7'th* event ever
recorded in that tenant.

The results of this command can be independently checked by downloading the
public verifiable data for the DataTrails tenant on which the event was
recorded.

```
curl -H "x-ms-blob-type: BlockBlob" -H "x-ms-version: 2019-12-12" https://app.datatrails.ai/verifiabledata/merklelogs/v1/mmrs/tenant/7dfaa5ef-226f-4f40-90a5-c015e59998a8/0/massifs/0000000000000000.log -o mmr.log
```

Using this [online hexeditor](https://hexed.it/) the `mmr.log` can be uploaded
and you can repeat the search performed above using its interface.

The format of the log is described in detail in ["Navigating the Merkle Logs"](https://docs.datatrails.ai/developers/developer-patterns/navigating-merklelogs/) (note: this material is not released yet)

# Verifying a single event

An example of verifying the following single event using api response data.

https://app.datatrails.ai/archivist/v2/publicassets/87dd2e5a-42b4-49a5-8693-97f40a5af7f8/events/a022f458-8e55-4d63-a200-4172a42fc2aa

We use a publicly attested event so that you can check the event details directly.

    EVENT_ID=publicassets/87dd2e5a-42b4-49a5-8693-97f40a5af7f8/events/a022f458-8e55-4d63-a200-4172a42fc2aa
    DATATRAILS_URL=https://app.datatrails.ai
    PUBLIC_TENANT_ID=tenant/6ea5cd00-c711-3649-6914-7b125928bbb4

    curl -sL $DATATRAILS_URL/archivist/v2/$EVENT_ID | \
        veracity --url $DATATRAILS_URL/verifiabledata --tenant=$PUBLIC_TENANT_ID events-verify

**By default there will be no output. If the verification has succeeded an exit code of 0 will be returned.**

If the verification command is run with `--log-level=INFO` the output will be:

    verifying for tenant: tenant/6ea5cd00-c711-3649-6914-7b125928bbb4
    verifying: 663 334 018fa97ef269039b00 2024-05-24T08:27:00.2+01:00 publicassets/87dd2e5a-42b4-49a5-8693-97f40a5af7f8/events/a022f458-8e55-4d63-a200-4172a42fc2aa
    leaf hash: bfc511ab1b880b24bb2358e07472e3383cdeddfbc4de9d66d652197dfb2b6633
    OK|663 334|[aea799fb2a8..., proof path nodes, ...f0a52d2256c235]


The elided proof path nodes will be:

    [9f0183c7f79fd81966e104520af0f90c8447f1a73d4e38e7f2f23a0602ceb617,
     da21cb383d63896a9811f06ebd2094921581d8eb72f7fbef566b730958dc35f1,
     51ea08fd02da3633b72ef0b09d8ba4209db1092d22367ef565f35e0afd4b0fc3,
     185a9d55cf507ef85bd264f4db7228e225032c48da689aa8597e11059f45ab30,
     bab40107f7d7bebfe30c9cea4772f9eb3115cae1f801adab318f90fcdc204bdc,
     94ca607094ead6fcd23f52851c8cdd8c6f0e2abde20dca19ba5abc8aff70d0d1,
     4b6dc2ff8d608faee7be16f900d58f7ff02360db319dc68f76035890d65c8c05,
     7fafc7edc434225afffc19b0582efa2a71b06a2d035358356df0a52d2256c235]

The same command accepts the result of a DataTrails list events call. And the event data can be supplied as local file if desired.

    curl -sL $DATATRAILS_URL/archivist/v2/$EVENT_ID > event.json
    veracity --url $DATATRAILS_URL/verifiabledata --tenant=$PUBLIC_TENANT_ID events-verify event.json

# General use commands

* `node` - read a merklelog node
* `nodescan` - scan a log for a particular node value
* `diag` - print diagnostics about a massif, identified by massif index or by an mmr index
* `events-verify` - verify the inclusion of an event, or list of events, in the tenant's merkle log
* `event-log-info` - print diagnostics about an events entry in the log (currently only supports events on protected assets)
* `massifs` - Generate pre-calculated tables for navigating massif raw storage with maximum convenience

# Developer commands

The following sub commands are used in development or by contributors. Or
currently require an authenticated connection

* tail, watch
