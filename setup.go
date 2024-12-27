package libbox

import (
	"bytes"
	"io"
	"os"
	"os/user"
	"path/filepath"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"github.com/nekohasekai/libwtf/internal/humanize"

	"github.com/sagernet/sing/common"
	F "github.com/sagernet/sing/common/format"
	"github.com/sagernet/sing/common/rw"
	"github.com/v2fly/v2ray-core/v5"
	"github.com/v2fly/v2ray-core/v5/common/platform/filesystem"
)

var (
	sBasePath        string
	sWorkingPath     string
	sTempPath        string
	sUserID          int
	sGroupID         int
	sTVOS            bool
	sFixAndroidStack bool
)

func init() {
	debug.SetPanicOnFault(true)
}

type SetupOptions struct {
	BasePath        string
	WorkingPath     string
	TempPath        string
	Username        string
	IsTVOS          bool
	FixAndroidStack bool
}

func Setup(options *SetupOptions) error {
	sBasePath = options.BasePath
	sWorkingPath = options.WorkingPath
	sTempPath = options.TempPath
	if options.Username != "" {
		sUser, err := user.Lookup(options.Username)
		if err != nil {
			return err
		}
		sUserID, _ = strconv.Atoi(sUser.Uid)
		sGroupID, _ = strconv.Atoi(sUser.Gid)
	} else {
		sUserID = os.Getuid()
		sGroupID = os.Getgid()
	}
	sTVOS = options.IsTVOS

	// TODO: remove after fixed
	// https://github.com/golang/go/issues/68760
	sFixAndroidStack = options.FixAndroidStack

	os.MkdirAll(sWorkingPath, 0o777)
	os.MkdirAll(sTempPath, 0o777)
	if options.Username != "" {
		os.Chown(sWorkingPath, sUserID, sGroupID)
		os.Chown(sTempPath, sUserID, sGroupID)
	}

	return nil
}

type AssetReader interface {
	ReadAsset(name string) ([]byte, error)
}

func SetupV2Ray(reader AssetReader) {
	filesystem.NewFileSeeker = func(path string) (io.ReadSeekCloser, error) {
		_, fileName := filepath.Split(path)
		workFile := filepath.Join(sWorkingPath, fileName)
		if rw.IsFile(workFile) {
			return os.Open(workFile)
		}
		var (
			content []byte
			err     error
		)
		if strings.HasSuffix(fileName, ".dat") {
			content, err = reader.ReadAsset(common.SubstringBefore(fileName, ".dat"))
		} else {
			content, err = reader.ReadAsset(fileName)
		}
		if err != nil {
			return nil, err
		}
		return struct {
			io.ReadSeeker
			io.Closer
		}{
			ReadSeeker: bytes.NewReader(content),
			Closer:     io.NopCloser(nil),
		}, nil
	}
	filesystem.NewFileReader = func(path string) (io.ReadCloser, error) {
		return filesystem.NewFileSeeker(path)
	}
}

func SetLocale(localeId string) {
	// not used by v2ray
}

func Version() string {
	return core.Version()
}

func FormatBytes(length int64) string {
	return humanize.Bytes(uint64(length))
}

func FormatMemoryBytes(length int64) string {
	return humanize.MemoryBytes(uint64(length))
}

func FormatDuration(durationInt int64) string {
	duration := time.Duration(durationInt) * time.Millisecond
	if duration < time.Second {
		return F.ToString(duration.Milliseconds(), "ms")
	} else if duration < time.Minute {
		return F.ToString(int64(duration.Seconds()), ".", int64(duration.Seconds()*100)%100, "s")
	} else {
		return F.ToString(int64(duration.Minutes()), "m", int64(duration.Seconds())%60, "s")
	}
}

func ProxyDisplayType(proxyType string) string {
	return proxyType
}
