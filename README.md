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
curl -sLO https://github.com/datatrails/veracity/releases/latest/download/veracity_${PLATFORM}_${ARCH}.tar.gz
tar -xf veracity_${PLATFORM}_${ARCH}.tar.gz
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

# Verifying a single event

An example of verifying the following single event using api response data.

https://app.datatrails.ai/archivist/v2/publicassets/87dd2e5a-42b4-49a5-8693-97f40a5af7f8/events/a022f458-8e55-4d63-a200-4172a42fc2aa

We use a publicly attested event so that you can check the event details directly.

    EVENT_ID=publicassets/87dd2e5a-42b4-49a5-8693-97f40a5af7f8/events/a022f458-8e55-4d63-a200-4172a42fc2aa
    DATATRAILS_URL=https://app.datatrails.ai
    PUBLIC_TENANT_ID=tenant/6ea5cd00-c711-3649-6914-7b125928bbb4

    curl -sL $DATATRAILS_URL/archivist/v2/$EVENT_ID | \
        veracity --data-url $DATATRAILS_URL/verifiabledata --tenant=$PUBLIC_TENANT_ID verify-included

**By default there will be no output. If the verification has succeeded an exit code of 0 will be returned.**

If the verification command is run with `--loglevel=INFO` the output will be:

    verifying for tenant: tenant/6ea5cd00-c711-3649-6914-7b125928bbb4
    verifying: 663 334 018fa97ef269039b00 2024-05-24T08:27:00.2+01:00 publicassets/87dd2e5a-42b4-49a5-8693-97f40a5af7f8/events/a022f458-8e55-4d63-a200-4172a42fc2aa
    leaf hash: bfc511ab1b880b24bb2358e07472e3383cdeddfbc4de9d66d652197dfb2b6633
    OK|663 334|[aea799fb2a8..., proof path nodes, ...f0a52d2256c235]


The elided proof path at time of writing was:

    [aea799fb2a8c4bbb6eda1dd2c1e69f8807b9b06deeaf51b9e0287492cefd8e4c, 9f0183c7f79fd81966e104520af0f90c8447f1a73d4e38e7f2f23a0602ceb617, da21cb383d63896a9811f06ebd2094921581d8eb72f7fbef566b730958dc35f1, 51ea08fd02da3633b72ef0b09d8ba4209db1092d22367ef565f35e0afd4b0fc3, 185a9d55cf507ef85bd264f4db7228e225032c48da689aa8597e11059f45ab30, bab40107f7d7bebfe30c9cea4772f9eb3115cae1f801adab318f90fcdc204bdc, 94ca607094ead6fcd23f52851c8cdd8c6f0e2abde20dca19ba5abc8aff70d0d1, ba6d0fd8922342aafbba6073c5510103b077a7de9cb2d72fb652510110250f9e, 7fafc7edc434225afffc19b0582efa2a71b06a2d035358356df0a52d2256c235, b737375d837e67ee7bce182377304e889187ef0f335952174cb5bf707a0b4788]

The same command accepts the result of a DataTrails list events call, e.g.

    DATATRAILS_URL=https://app.datatrails.ai
    PUBLIC_TENANT_ID=tenant/6ea5cd00-c711-3649-6914-7b125928bbb4
    PUBLIC_ASSET_ID=publicassets/87dd2e5a-42b4-49a5-8693-97f40a5af7f8

    curl -sL $DATATRAILS_URL/archivist/v2/$PUBLIC_ASSET_ID/events | \
      veracity --data-url $DATATRAILS_URL/verifiabledata --tenant=$PUBLIC_TENANT_ID verify-included 

# Read selected node from log

An example of reading node associated with specific event, it's possible to visit merkle log entry page https://app.datatrails.ai/merklelogentry/87dd2e5a-42b4-49a5-8693-97f40a5af7f8/999773ed-cc92-4d9c-863f-b418418705ea?public=true for event https://app.datatrails.ai/archivist/publicassets/87dd2e5a-42b4-49a5-8693-97f40a5af7f8/events/999773ed-cc92-4d9c-863f-b418418705ea

On the Merkle log entry page we can see `MMR Index` field with value `916` which can be used with `node` command to retrieve the leaf directly from merklelog by using following command

    PUBLIC_TENANT_ID=tenant/6ea5cd00-c711-3649-6914-7b125928bbb4
    DATATRAILS_URL=https://app.datatrails.ai
 
    veracity --data-url $DATATRAILS_URL/verifiabledata --tenant=$PUBLIC_TENANT_ID node --mmrindex 916

Above command will output `c3323019fd1d325ac068d203c62007b504c5fa762446a9fe5d88e392ec96914b` which will match the value from the merkle log enty page.

# General use commands

* `node` - read a merklelog node
* `verify-included` - verify the inclusion of an event, or list of events, in the tenant's merkle log
