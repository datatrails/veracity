package veracity

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/datatrails/go-datatrails-common/azblob"
	"github.com/datatrails/go-datatrails-common/logger"
	"github.com/datatrails/go-datatrails-merklelog/massifs"
)

type Opener interface {
	Open(string) (io.ReadCloser, error)
}

type DirLister interface {
	// ListFiles returns list of absolute paths
	// to files (not subdirectories) in a directory
	ListFiles(string) ([]string, error)
}

type LocalMassifReader struct {
	log              logger.Logger
	opts             massifs.MassifReaderOptions
	massifs          map[uint64]string
	headMassifIndex  uint64
	firstMassifIndex uint64
	opener           Opener
	dirLister        DirLister
}

func (mr *LocalMassifReader) GetMassif(
	ctx context.Context, tenantIdentity string, massifIndex uint64,
	opts ...azblob.Option,
) (massifs.MassifContext, error) {

	var err error
	mc := massifs.MassifContext{
		TenantIdentity: tenantIdentity,
	}
	if err = mr.readAndPrepareContext(&mc, massifIndex); err != nil {
		return massifs.MassifContext{}, err
	}
	return mc, nil
}

func (mr *LocalMassifReader) GetHeadMassif(
	ctx context.Context, tenantIdentity string,
	opts ...azblob.Option,
) (massifs.MassifContext, error) {

	var err error
	mc := massifs.MassifContext{
		TenantIdentity: tenantIdentity,
	}
	if len(mr.massifs) == 0 {
		return massifs.MassifContext{}, massifs.ErrMassifNotFound
	}
	if err = mr.readAndPrepareContext(&mc, mr.headMassifIndex); err != nil {
		return massifs.MassifContext{}, err
	}

	return mc, nil
}

// GetLazyContext is an optimization for remote massif readers
// and is therefor not implemented for local massif reader
func (mr *LocalMassifReader) GetLazyContext(
	ctx context.Context, tenantIdentity string, which massifs.LogicalBlob,
	opts ...azblob.Option,
) (massifs.LogBlobContext, uint64, error) {

	return massifs.LogBlobContext{}, 0, fmt.Errorf("not implemented for local storage")
}

func (mr *LocalMassifReader) GetFirstMassif(
	ctx context.Context, tenantIdentity string,
	opts ...azblob.Option,
) (massifs.MassifContext, error) {

	var err error
	mc := massifs.MassifContext{
		TenantIdentity: tenantIdentity,
	}
	if len(mr.massifs) == 0 {
		return massifs.MassifContext{}, massifs.ErrMassifNotFound
	}
	if err = mr.readAndPrepareContext(&mc, mr.firstMassifIndex); err != nil {
		return massifs.MassifContext{}, err
	}

	return mc, nil
}

// NewLocalMassifReader creates MassifReader that reads from
// local files on disc - it mostly ignores tenant identity
// as we assume all the logs on the disc are for the tenant one is
// interested in - but it's still valid to pass tenant ID here
func NewLocalMassifReader(
	log logger.Logger, opener Opener, dirLister DirLister, logdir string, logfile string, opts ...massifs.MassifReaderOption,
) (*LocalMassifReader, error) {
	r := LocalMassifReader{
		log:              log,
		opener:           opener,
		dirLister:        dirLister,
		massifs:          map[uint64]string{},
		firstMassifIndex: ^uint64(0), //set to max so we can lower it as we find new logs
	}

	for _, o := range opts {
		o(&r.opts)
	}

	if logdir != "" && logfile != "" {
		return nil, fmt.Errorf("logfile and logdir can't be used together")
	}

	if logdir != "" {
		err := r.findLogfiles(logdir)
		if err != nil {
			return nil, err
		}
	}

	if logfile != "" {
		err := r.loadLogfile(logfile)
		if err != nil {
			return nil, err
		}
	}

	return &r, nil
}

func (mr *LocalMassifReader) findLogfiles(directory string) error {
	// read all the entries in our log dir
	entries, err := mr.dirLister.ListFiles(directory)
	if err != nil {
		return err
	}

	// for each entry we read the header (first 32 bytes)
	// and do rough checks if the header looks like it's from a valid log
	for _, filepath := range entries {
		err := mr.loadLogfile(filepath)
		if err != nil {
			return err
		}
	}
	return nil
}

func (mr *LocalMassifReader) loadLogfile(logfile string) error {
	f, err := mr.opener.Open(logfile)
	if err != nil {
		return err
	}
	defer f.Close()
	header := make([]byte, 32)

	i, err := f.Read(header)
	if err != nil && err != io.EOF {
		return err
	}

	// if we read less than 32 bytes we ignore the file completely
	// as it's not a valid log
	if i != 32 {
		mr.log.Debugf("could not read enough bytes from a file: %s", logfile)
		return nil
	}

	// unmarshal the header
	ms := massifs.MassifStart{}
	err = massifs.DecodeMassifStart(&ms, header)
	if err != nil {
		return err
	}

	// check if the massif height is what we expect - if it's not we ignore the file
	if ms.MassifHeight != defaultMassifHeight {
		mr.log.Debugf("got unexpected massif height of %d", ms.MassifHeight)
		return nil
	}

	// if we already have a log with the same massifIndex we
	// error out as the files in directories are potentially not for the
	// same tenancy - which means the data is not correct
	if fname, ok := mr.massifs[uint64(ms.MassifIndex)]; ok {
		return fmt.Errorf("found two log files with the same massif index: %s and %s", fname, logfile)
	}

	// associate filename with the massif index
	mr.massifs[uint64(ms.MassifIndex)] = logfile

	// update the head massif index if we have new one
	if uint64(ms.MassifIndex) > mr.headMassifIndex {
		mr.headMassifIndex = uint64(ms.MassifIndex)
	}

	// update the first massif index if we have new one
	if uint64(ms.MassifIndex) < mr.firstMassifIndex {
		mr.firstMassifIndex = uint64(ms.MassifIndex)
	}

	return nil
}

func (mr *LocalMassifReader) readAndPrepareContext(mc *massifs.MassifContext, massifIndex uint64) error {
	var err error
	var ok bool
	var fileName string
	// check if massif with particular index was found
	if fileName, ok = mr.massifs[massifIndex]; !ok {
		return fmt.Errorf("could not find log for massif Index of %d", massifIndex)
	}

	reader, err := mr.opener.Open(fileName)
	if err != nil {
		return err
	}
	defer reader.Close()

	// read the data from a file
	mc.Data, err = io.ReadAll(reader)
	if err != nil {
		return err
	}

	// unmarshal
	err = mc.Start.UnmarshalBinary(mc.Data)
	if err != nil {
		return err
	}

	return nil
}

// Utilities to remove the os dependencies from the MassifReader
type OsDirLister struct{}

func (*OsDirLister) ListFiles(name string) ([]string, error) {
	dpath, err := filepath.Abs(name)
	if err != nil {
		return nil, err
	}
	result := []string{}
	entries, err := os.ReadDir(dpath)
	if err != nil {
		return result, err
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			result = append(result, filepath.Join(dpath, entry.Name()))
		}
	}
	return result, nil
}

func cfgDirLister() DirLister {
	return &OsDirLister{}
}

type FileOpener struct{}

func (*FileOpener) Open(name string) (io.ReadCloser, error) {
	fpath, err := filepath.Abs(name)
	if err != nil {
		return nil, err
	}
	return os.Open(fpath)
}

func cfgOpener() Opener {
	return &FileOpener{}
}
