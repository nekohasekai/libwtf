package libbox

import (
	"os"
	"runtime"

	F "github.com/sagernet/sing/common/format"
	"github.com/v2fly/v2ray-core/v5/common/log"
	"golang.org/x/sys/unix"
)

type commandServerLogger struct {
	*CommandServer
}

func (l *commandServerLogger) Handle(msg log.Message) {
	l.WriteMessage(msg.String())
}

type stubLogger struct{}

func (l *stubLogger) Handle(msg log.Message) {
}

type v2rayLogger struct{}

func (l *v2rayLogger) Trace(args ...any) {
	log.Record(&log.GeneralMessage{
		Severity: log.Severity_Debug,
		Content:  F.ToString(args...),
	})
}

func (l *v2rayLogger) Debug(args ...any) {
	log.Record(&log.GeneralMessage{
		Severity: log.Severity_Debug,
		Content:  F.ToString(args...),
	})
}

func (l *v2rayLogger) Info(args ...any) {
	log.Record(&log.GeneralMessage{
		Severity: log.Severity_Info,
		Content:  F.ToString(args...),
	})
}

func (l *v2rayLogger) Warn(args ...any) {
	log.Record(&log.GeneralMessage{
		Severity: log.Severity_Warning,
		Content:  F.ToString(args...),
	})
}

func (l *v2rayLogger) Error(args ...any) {
	log.Record(&log.GeneralMessage{
		Severity: log.Severity_Error,
		Content:  F.ToString(args...),
	})
}

func (l *v2rayLogger) Fatal(args ...any) {
	log.Record(&log.GeneralMessage{
		Severity: log.Severity_Error,
		Content:  F.ToString(args...),
	})
}

func (l *v2rayLogger) Panic(args ...any) {
	log.Record(&log.GeneralMessage{
		Severity: log.Severity_Error,
		Content:  F.ToString(args...),
	})
}

var stderrFile *os.File

func RedirectStderr(path string) error {
	if stats, err := os.Stat(path); err == nil && stats.Size() > 0 {
		_ = os.Rename(path, path+".old")
	}
	outputFile, err := os.Create(path)
	if err != nil {
		return err
	}
	if runtime.GOOS != "android" {
		err = outputFile.Chown(sUserID, sGroupID)
		if err != nil {
			outputFile.Close()
			os.Remove(outputFile.Name())
			return err
		}
	}
	err = unix.Dup2(int(outputFile.Fd()), int(os.Stderr.Fd()))
	if err != nil {
		outputFile.Close()
		os.Remove(outputFile.Name())
		return err
	}
	stderrFile = outputFile
	return nil
}
