package veracity

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/datatrails/go-datatrails-common/logger"
	"github.com/datatrails/go-datatrails-logverification/logverification/app"
	"github.com/datatrails/go-datatrails-merklelog/massifs"
	"github.com/datatrails/go-datatrails-merklelog/mmr"
	"github.com/datatrails/go-datatrails-serialization/eventsv1"
	veracityapp "github.com/datatrails/veracity/app"
	"github.com/urfave/cli/v2"
)

/**
 * find mmr Entries finds the mmr entry associated with given app data
 */

const (
	appEntryFileFlagName = "app-entry-file"
)

// findMMREntries searchs the log of the given log tenant for matching mmrEntries given the app entries
// and returns the leaf indexes of all the matches as well as the number of mmr entries considered
func findMMREntries(
	log logger.Logger,
	massifReader MassifReader,
	tenantLogPath string,
	massifStartIndex int64,
	massifEndIndex int64,
	massifHeight uint8,
	appEntries ...[]byte,
) ([]uint64, uint64, error) {

	// find the starting leaf index by finding the number of leaf nodes in a full massif, of the given massif height,
	//  then multiplying that by the number of massifs we are skipping over
	leafIndex := uint64(massifStartIndex * int64(mmr.HeightIndexLeafCount(uint64(massifHeight-1))))

	matchingLeafIndexes := []uint64{}
	entriesConsidered := uint64(0)

	massifIndex := massifStartIndex

	// search all massifs from the starting index to the end index
	for {

		// check if we have reached the last massif we want to consider checking.
		// -1 means check until the last massif, so never break here if massifEndIndex == -1
		if massifIndex > massifEndIndex && massifEndIndex != -1 {
			break
		}

		massifContext, err := massifReader.GetMassif(context.Background(), tenantLogPath, uint64(massifIndex))

		// check if we have reached the last massif for the log tenant
		if errors.Is(err, massifs.ErrMassifNotFound) {
			break
		}

		// check if we have reached the last massif for local log
		if errors.Is(err, massifs.ErrLogFileMassifNotFound) {
			break
		}

		// check if we get an azblob error of blob not found
		// this is also an indication we have reached the last massif.
		//
		// NOTE: due to the azblob error type we need to do string contains.
		if err != nil && strings.Contains(err.Error(), "BlobNotFound") {
			break
		}

		// any other error we can error out on.
		if err != nil {
			return nil, 0, err
		}

		// get the mmrLeafEntries count based on the size
		// NOTE: the leaf count and trie entry count are the same
		// NOTE: the leaf index and trie index are equivilent.
		mmrLeafEntries := massifContext.MassifLeafCount()

		log.Debugf("checking %v mmr entries in massif %v for matches", mmrLeafEntries, massifIndex)

		// check each mmr leaf entry for matching mmr entry
		for range mmrLeafEntries {

			// we increment leafIndex at the end of the loop, because we have 2 loops
			//  and we want the leaf index to continue and not reset each inner loop.
			mmrIndex := mmr.MMRIndex(leafIndex)

			// get the mmrEntry from the massif
			logMMREntry, err := massifContext.Get(mmrIndex)
			if err != nil {
				return nil, 0, err
			}

			// find the mmr entry from the given app entries
			logTrieEntry, err := massifContext.GetTrieEntry(mmrIndex)
			if err != nil {
				return nil, 0, err
			}

			extraBytes := massifs.GetExtraBytes(logTrieEntry, 0, 0)

			for _, appEntry := range appEntries {

				var serializedBytes []byte

				// app domain 0 is assetsv2
				if extraBytes[0] == 0 {
					serializedBytes = appEntry
				}

				// app domain 1 is eventsv1
				if extraBytes[0] == 1 {
					serializedBytes, err = eventsv1.SerializeEventFromJson(appEntry)
					if err != nil {
						return nil, 0, err
					}
				}

				entry := app.NewAppEntry(
					"",
					[]byte{},
					app.NewMMREntryFields(
						0,
						serializedBytes,
					),
					mmrIndex,
				)

				// find the mmr entry from the given app entry
				derivedMMREntry, err := entry.MMREntry(&massifContext)
				if err != nil {
					// NOTE: it is possible that the log entry is assetsv2
					//       but we are searching for an eventsv2 event or vice versa
					//       so we shouldn't return on error, just continue as we know
					//       its not a match.
					//
					// NOTE: we should do better error handling here and ensure we handle
					//       transient errors like failing to get idtimestamp or extrabytes
					//       from the log
					continue
				}

				// compare the mmr entry from the log to the derived mmr entry
				//  from the given app entry
				if bytes.Equal(logMMREntry, derivedMMREntry) {
					// match
					matchingLeafIndexes = append(matchingLeafIndexes, leafIndex)
				}

			}

			leafIndex++
			entriesConsidered++

		}

		massifIndex++
	}

	return matchingLeafIndexes, entriesConsidered, nil

}

// NewFindMMREntriesCmd finds the mmr entries associated with a given app entries in the tenants Merkle Log.
//
//nolint:gocognit
func NewFindMMREntriesCmd() *cli.Command {
	return &cli.Command{
		Name: "find-mmr-entries",
		Usage: `finds the matching mmr entries for the given app entry.

		By default returns all mmr Indexes of matching mmr entries.

		The mmr entry is HASH(DOMAIN | MMR SALT | APP ENTRY)

		NOTE: ignores the global --tenant option, please use --log-tenant command option.
`,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     logTenantFlagName,
				Usage:    "the tenant of the log to search in. Required",
				Required: true,
			},
			&cli.StringFlag{
				Name:  appEntryFileFlagName,
				Usage: "the file containing the app entry, if omitted uses stdin.",
				Value: "",
			},
			&cli.BoolFlag{
				Name:  asLeafIndexesFlagName,
				Usage: "if true, returns a list of matching leaf indexes instead of mmr indexes.",
				Value: false,
			},
			&cli.Int64Flag{
				Name:  massifRangeStartFlagName,
				Usage: "if set, start the search for matching mmr entries at the massif at this given massif index. if omitted will start search at massif 0.",
				Value: 0,
			},
			&cli.Int64Flag{
				Name:  massifRangeEndFlagName,
				Usage: "if set, end the search for matching mmr entries at the massif at this given massif index. if omitted will end search at the last massif.",
				Value: -1,
			},
		},
		Action: func(cCtx *cli.Context) error {
			cmd := &CmdCtx{}

			// This command uses the structured logger for all optional output.
			if err := cfgLogging(cmd, cCtx); err != nil {
				return err
			}

			// get all flags
			logTenant := cCtx.String(logTenantFlagName)

			appEntryFileName := cCtx.String(appEntryFileFlagName)

			appEntry, err := veracityapp.ReadAppData(appEntryFileName == "", appEntryFileName)
			if err != nil {
				return err
			}

			asLeafIndexes := cCtx.Bool(asLeafIndexesFlagName)

			massifStartIndex := cCtx.Int64(massifRangeStartFlagName)
			massifEndIndex := cCtx.Int64(massifRangeEndFlagName)

			// If we are reading the massif log locally, the log path is the
			// data-local path. The reader does the right thing regardless of
			// whether the option is a directory or a file.
			// verifyEvent defaults it to tenantIdentity for the benefit of the remote reader implementation
			tenantLogPath := cCtx.String("data-local")

			if tenantLogPath == "" {
				tenantLogPath = logTenant
			}

			// configure the cmd massif reader
			if err = cCtx.Set("tenant", logTenant); err != nil {
				return err
			}

			if err = cfgMassifReader(cmd, cCtx); err != nil {
				return err
			}

			cmd.log.Debugf("app entry: %x", appEntry)

			leafIndexMatches, entriesConsidered, err := findMMREntries(
				cmd.log,
				cmd.massifReader,
				tenantLogPath,
				massifStartIndex,
				massifEndIndex,
				cmd.massifHeight,
				appEntry,
			)
			if err != nil {
				return err
			}

			cmd.log.Debugf("entries considered: %v", entriesConsidered)

			// if we want the leaf index matches log them and return
			if asLeafIndexes {
				fmt.Printf("matches: %v\n", leafIndexMatches)
				return nil
			}

			// otherwise we want to log the mmr index matches
			mmrIndexMatches := []uint64{}
			for _, leafIndex := range leafIndexMatches {

				mmrIndex := mmr.MMRIndex(leafIndex)
				mmrIndexMatches = append(mmrIndexMatches, mmrIndex)
			}

			fmt.Printf("matches: %v\n", mmrIndexMatches)

			return nil

		},
	}
}
