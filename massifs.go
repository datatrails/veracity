package veracity

import (
	"fmt"
	"strings"

	"github.com/datatrails/forestrie/go-forestrie/massifs"
	"github.com/datatrails/forestrie/go-forestrie/mmr"
	"github.com/urfave/cli/v2"
)

const (
	plainFmtName = "plain"
	tableFmtName = "table"
)

// NewMassifsCmd prints out pre-calculated tables for navigating massif blobs
// with maximum convenience
func NewMassifsCmd() *cli.Command {
	return &cli.Command{Name: "massifs",
		Usage: `
Generate pre-calculated tables for navigating massif blobs with maximum convenience.

Note that this command does not need to read any blobs. It simply applies the
MMR algorithms to produce the desired information computationaly Note that this
command does not need to read any blobs. It simply applies the MMR algorithms
to produce the desired information computationaly produce`,
		Flags: []cli.Flag{
			&cli.Uint64Flag{
				Name: "first-massif", Aliases: []string{"m"},
				Usage: "The massif to start at",
				Value: 0,
			},
			&cli.Uint64Flag{
				Name: "mmrindex", Aliases: []string{"i"},
				Usage: "Chose the massif to start at by finding the massif containing this node. Takes precedence over --first-massif",
				Value: 0,
			},
			&cli.Uint64Flag{
				Name: "count", Aliases: []string{"n"},
				Usage: "The number of massifs to produce table rows for",
				Value: 1,
			},
			&cli.StringFlag{
				Name: "format", Aliases: []string{"f"},
				Value: plainFmtName,
				Usage: fmt.Sprintf("table format to use. one of [%s, %s]", plainFmtName, tableFmtName),
			},
			&cli.BoolFlag{
				Name:  "sizes",
				Usage: "Set to additionally report the sizes of each variable section and the trie data",
			},
		},
		Action: func(cCtx *cli.Context) error {
			var err error

			cmd := &CmdCtx{}

			// Note that this command does not need to read any blobs. It simply
			// applies the MMR algorithms to produce the desired information
			// computationaly
			if err = cfgLogging(cmd, cCtx); err != nil {
				return err
			}

			height := uint8(cCtx.Uint("height"))
			if height < 1 {
				return fmt.Errorf("massif height must be > 0")
			}

			// support identifying the massif implicitly via the index of a log
			// entry. note that mmrIndex 0 is just handled as though the caller
			// asked for massifIndex 0
			mmrIndex := cCtx.Uint64("mmrindex")
			massifIndex := cCtx.Uint64("first-massif")
			if mmrIndex > uint64(0) {
				massifIndex, err = massifs.MassifIndexFromMMRIndex(height, mmrIndex)
				if err != nil {
					return err
				}
			}

			count := cCtx.Uint64("count")

			// The massif height log configuration parameter defines the fixed number
			// of leaves in each blob.
			massifLeafCount := mmr.HeightIndexLeafCount(uint64(height) - 1)

			// // Given our desired start massif index, workout the first and last
			// // leaf indices included in the start massif
			// firstLeaf := massifIndex * massifLeafCount
			// lastLeaf := firstLeaf + massifLeafCount - 1
			// // Use the log base 2 algorithm to compute the first and last mmrIndex from the first
			// // and last leaf indices
			// firstMMRIndex := mmr.TreeIndex(firstLeaf)
			// lastMMRIndex := mmr.TreeIndex(lastLeaf)

			var peakStackIndices []uint64

			for mi := massifIndex; mi < (massifIndex + count); mi++ {

				firstLeaf := mi * massifLeafCount
				lastLeaf := firstLeaf + massifLeafCount - 1
				firstMMRIndex := mmr.TreeIndex(firstLeaf)
				lastMMRIndex := mmr.TreeIndex(lastLeaf+1) - 1

				// It's retrospective, stored in the current massif based on the
				// *last* full massif. which we always  know when starting a new
				// massif.
				peakStackIndices = PeakStack(height, firstMMRIndex+1)
				peakStackStart := massifs.PeakStackStart(height)
				logStart := massifs.PeakStackEnd(mi, height)

				tableFmt := "|% 8d|% 8d|% 8d|% 8d|% 8d|% 8d|% 8d| [%s]"
				plainFmt := "% 8d% 8d% 8d% 8d% 8d% 8d% 8d [%s]"

				// trieDataSize, peakStackSize, mmrDataSize
				tableSizesFmt := "|% 8d|% 8d"
				plainSizesFmt := "% 8d% 8d%"

				var row string
				switch cCtx.String("format") {
				case tableFmtName:
					row = fmt.Sprintf(tableFmt, mi, peakStackStart, logStart, firstLeaf, lastLeaf, firstMMRIndex, lastMMRIndex, joinStack(peakStackIndices))
				case plainFmtName:
					row = fmt.Sprintf(plainFmt, mi, peakStackStart, logStart, firstLeaf, lastLeaf, firstMMRIndex, lastMMRIndex, joinStack(peakStackIndices))
				default:
					row = fmt.Sprintf(plainFmt, mi, peakStackStart, logStart, firstLeaf, lastLeaf, firstMMRIndex, lastMMRIndex, joinStack(peakStackIndices))
				}
				if cCtx.Bool("sizes") {
					// this calc is wrong (its double) but it is what we do. the
					// massifs all have twice the amount of space reserved for
					// the trie than we need.
					trieDataSize := massifs.TrieEntryBytes * (1 << height)
					peakStackSize := massifs.PeakStackLen(mi) * 32
					switch cCtx.String("format") {
					case tableFmtName:
						row = fmt.Sprintf(tableSizesFmt, trieDataSize, peakStackSize) + row
					case plainFmtName:
						row = fmt.Sprintf(plainSizesFmt, trieDataSize, peakStackSize) + row
					default:
						row = fmt.Sprintf(plainSizesFmt, trieDataSize, peakStackSize) + row
					}
				}
				fmt.Println(row)
			}

			return nil
		},
	}
}

func joinStack(stackIndices []uint64) string {
	if stackIndices == nil {
		return ""
	}
	var s []string
	for _, pi := range stackIndices {
		s = append(s, fmt.Sprintf("%d", pi))
	}
	return strings.Join(s, ",")
}

// PeakStack returns the stack of mmrIndices corresponding to the stack of
// ancestor nodes required for mmrSize. Note that the trick here is to realise
// that passing a massifIndex+1 in place of mmrSize, treating each massif as a
// leaf node in a much smaller tree, gets the (much shorter) peak stack of
// nodes required from earlier massifs. And this is stack of nodes carried
// forward in each massif blob to make them self contained.
// (The mmrblobs package has a slightly different variant of this that returns
// a map)
func PeakStack(massifHeight uint8, mmrSize uint64) []uint64 {
	var stack []uint64
	iPeaks := mmr.Peaks(mmrSize)
	for _, ip := range iPeaks {
		if mmr.PosHeight(ip) < uint64(massifHeight)-1 {
			continue
		}
		// remembering that Peaks returns *positions*
		stack = append(stack, ip-1)
	}
	return stack
}
