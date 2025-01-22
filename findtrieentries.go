package veracity

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/datatrails/go-datatrails-common/logger"
	"github.com/datatrails/go-datatrails-merklelog/massifs"
	"github.com/datatrails/go-datatrails-merklelog/mmr"
	"github.com/google/uuid"
	"github.com/urfave/cli/v2"
)

/**
 * find trie Entries finds the trie entry associated with a given trie key
 */

const (
	logTenantFlagName = "log-tenant"

	logIDFlagName = "log-id"

	domainFlagName = "domain"

	appIDFlagName = "app-id"

	asMMRIndexesFlagName = "as-mmrindexes"
)

var (
	ErrNoLogTenant = fmt.Errorf("error, cannot find log tenant, please provide either %v or %v", logIDFlagName, logTenantFlagName)
)

// logIDToLogTenant converts the log id to the log tenant
func logIDToLogTenant(logID string) (string, error) {

	// first get the byte representation of the hex
	logIdBytes, err := hex.DecodeString(logID)
	if err != nil {
		return "", err
	}

	// we don't know if its a log version 0 log id or a log version 1 log id

	// attempt log version 1 first

	// log version 1 is the byte representation of the uuid part of the log tenant
	logIdUUid, err := uuid.ParseBytes(logIdBytes)
	if err != nil {

		// assume its log version 0 if it can't be parsed as bytes
		// which is just utf-8 bytes of the log tenant string
		return string(logIdBytes), nil
	}

	// if we get here we know its log version 1, so make the log tenant from the uuid
	logTenant := fmt.Sprintf("tenant/%s", logIdUUid.String())

	return logTenant, nil
}

// findTrieKeys searchs the log of the given log tenant for matches to the given triekeys
// and returns the trie indexes (leaf indexes) of all the matches as well as the number of trie entries considered
func findTrieKeys(log logger.Logger, massifReader MassifReader, logTenant string, trieKeys ...[]byte) ([]uint64, uint64, error) {

	// get the head massif
	headMassifContext, err := massifReader.GetHeadMassif(context.Background(), logTenant)
	if err != nil {
		return nil, 0, err
	}

	// find the number of massifs
	massifCount := headMassifContext.Start.MassifIndex + 1

	trieIndex := uint64(0)

	matchingTrieIndexes := []uint64{}

	for massifIndex := range massifCount {

		massifContext, err := massifReader.GetMassif(context.Background(), logTenant, uint64(massifIndex))
		if err != nil {
			return nil, 0, err
		}

		// get the trieEntry count based on the size
		// NOTE: the leaf count and trie entry count are the same
		// NOTE: the leaf index and trie index are equivilent.
		trieEntries := massifContext.MassifLeafCount()

		log.Debugf("checking %v trie entries in massif %v for matches", trieEntries, massifIndex)

		// check each trie entry for matching trieKeys
		for range trieEntries {

			mmrIndex := mmr.MMRIndex(trieIndex)

			logTrieKey, err := massifContext.GetTrieKey(mmrIndex)
			if err != nil {
				return nil, 0, err
			}

			for _, trieKey := range trieKeys {

				// if a triekey matches add it to the matching trie indexes
				if bytes.Equal(trieKey, logTrieKey) {
					matchingTrieIndexes = append(matchingTrieIndexes, trieIndex)
					break // only one trieKey will ever match, so if we found the matching trie key, don't keep looking
				}

			}

			trieIndex++

		}
	}

	// the trie index is now the trie size as we do an extra +1 at the end of the for loop
	return matchingTrieIndexes, trieIndex, nil

}

// NewFindTrieEntriesCmd finds the trie entries associated with a given trie key in the tenants Merkle Log.
//
//nolint:gocognit
func NewFindTrieEntriesCmd() *cli.Command {
	return &cli.Command{
		Name: "find-trie-entries",
		Usage: `finds the matching trie entries for the given trie key.

		By default returns all trie Indexes (leaf indexes) of matching trie entries.

		The trieKey is HASH(DOMAIN | LOGID | APPID)

		NOTE: ignores the global --tenant option, please use --log-tenant command option.
`,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  logTenantFlagName,
				Usage: fmt.Sprintf("the tenant of the log to search in. Required or can be derived from %v.", logIDFlagName),
			},
			&cli.StringFlag{
				Name:  logIDFlagName,
				Usage: fmt.Sprintf("the hexadecimal representation of the log ID. Required or can be derived from %v.", logTenantFlagName),
			},
			&cli.StringFlag{
				Name:     appIDFlagName,
				Usage:    "the app ID. For eventsv1 or assetsv2 this is the event id.",
				Required: true,
			},
			&cli.Uint64Flag{
				Name:  domainFlagName,
				Usage: "the domain used to derive the trie key.",
				Value: uint64(massifs.KeyTypeApplicationContent),
			},
			&cli.BoolFlag{
				Name:  asMMRIndexesFlagName,
				Usage: "if true, returns a list of matching mmr indexes instead of trie indexes.",
				Value: false,
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
			logID := cCtx.String(logIDFlagName)

			appID := cCtx.String(appIDFlagName)

			domain := cCtx.Uint64(domainFlagName)

			asMMRIndexes := cCtx.Bool(asMMRIndexesFlagName)

			// check we only have at least 1 of log tenant or logID
			if logTenant == "" && logID == "" {
				return ErrNoLogTenant
			}

			// if we don't have the log tenant derive it from the
			//  log id
			if logTenant == "" {
				var err error
				logTenant, err = logIDToLogTenant(logID)
				if err != nil {
					return err
				}
			}

			// configure the cmd massif reader
			if err := cfgMassifReader(cmd, cCtx); err != nil {
				return err
			}

			trieIndexMatches := []uint64{}
			entriesConsidered := uint64(0)

			// if we have the logID use it to find the trie key.
			if logID != "" {

				logIDBytes, err := hex.DecodeString(logID)
				if err != nil {
					return err
				}

				trieKey := massifs.NewTrieKey(
					massifs.KeyType(domain),
					logIDBytes,
					[]byte(appID),
				)

				cmd.log.Debugf("trieKey: %x", trieKey)

				trieIndexMatches, entriesConsidered, err = findTrieKeys(cmd.log, cmd.massifReader, logTenant, trieKey)
				if err != nil {
					return err
				}

			}

			// if we don't have the trieKey we can derive it from the log tenant, but
			// we don't know if its log version 0 or log version 1, so do both.
			if logID == "" {

				logIDVersion0 := []byte(logTenant)

				trieKeyVersion0 := massifs.NewTrieKey(
					massifs.KeyType(domain),
					logIDVersion0,
					[]byte(appID),
				)

				cmd.log.Debugf("trieKey version 0: %x", trieKeyVersion0)

				logTenantUUIDStr := strings.TrimPrefix(logTenant, "tenant/")
				logTenantUUID, err := uuid.Parse(logTenantUUIDStr)
				if err != nil {

					// we could continue with just version 0 here
					// but if we error here we know the log tenant isn't a valid
					// tenant identity, so there is no point searching for the trie key
					// as we know its invalid.
					return err
				}

				logIDVersion1, err := logTenantUUID.MarshalBinary()
				if err != nil {
					return err
				}

				trieKeyVersion1 := massifs.NewTrieKey(
					massifs.KeyType(domain),
					logIDVersion1,
					[]byte(appID),
				)

				cmd.log.Debugf("trieKey version 1: %x", trieKeyVersion1)

				trieIndexMatches, entriesConsidered, err = findTrieKeys(cmd.log, cmd.massifReader, logTenant, trieKeyVersion0, trieKeyVersion1)
				if err != nil {
					return err
				}

			}

			cmd.log.Debugf("entries considered: %v", entriesConsidered)

			// if we want the trie index matches log them and return
			if !asMMRIndexes {
				fmt.Printf("matches: %v\n", trieIndexMatches)
				return nil
			}

			// otherwise we want to log the mmr index matches
			mmrIndexMatches := []uint64{}
			for _, trieIndex := range trieIndexMatches {

				mmrIndex := mmr.MMRIndex(trieIndex)
				mmrIndexMatches = append(mmrIndexMatches, mmrIndex)
			}

			fmt.Printf("matches: %v\n", mmrIndexMatches)

			return nil

		},
	}
}
