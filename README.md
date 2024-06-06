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

For example, for the linux or darwin the following steps would be conventional

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

This command will report the mmr index of that leaf as `10`

The conventional way to visualise the mmr index is like this

```

     6
   /  \
  2    5     9
 /\   / \   / \  
0  1  3  4 7  8  10  MMR INDEX

0  1  2  3 5  5   6 LEAF INDEX
```

And that shows that the leaf, which has mmr index `10` is the *6'th* event ever
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

# General use commands

* `node` - read a merklelog node
* `nodescan` - scan a log for a particular node value
* `diag` - print diagnostics about a massif, identified by massif index or by an mmr index
* `ediag` - print diagnostics about an events entry in the log (currently only supports events on protected assets)
* `massifs` - Generate pre-calculated tables for navigating massif raw storage with maximum convenience

# Developer commands

The following sub commands are used in development or by contributors. Or
currently require an authenticated connection

* tail, watch, prove
