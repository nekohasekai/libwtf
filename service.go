package libbox

import (
	"context"
	"os"
	runtimeDebug "runtime/debug"
	"time"

	_ "github.com/sagernet/gomobile"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/v2fly/v2ray-core/v5"
	_ "github.com/v2fly/v2ray-core/v5/main/distro/all"
)

type Service struct {
	ctx      context.Context
	cancel   context.CancelFunc
	instance *core.Instance
	tun      *tun2ray
}

func NewService(configContent string, platformInterface PlatformInterface) (*Service, error) {
	config, err := parseConfig(configContent)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	instance, err := core.NewWithContext(ctx, config)
	if err != nil {
		cancel()
		return nil, E.Cause(err, "create service")
	}
	return &Service{
		ctx:      ctx,
		cancel:   cancel,
		instance: instance,
		tun:      newTun2ray(ctx, instance, platformInterface),
	}, nil
}

func (s *Service) Start() error {
	if sFixAndroidStack {
		var err error
		done := make(chan struct{})
		go func() {
			err = s.instance.Start()
			close(done)
		}()
		<-done
		if err != nil {
			return err
		}
	} else {
		err := s.instance.Start()
		if err != nil {
			return err
		}
	}
	runtimeDebug.FreeOSMemory()
	return s.tun.Start()
}

func (s *Service) Close() error {
	const FatalStopTimeout = 10 * time.Second
	s.cancel()
	var err error
	done := make(chan struct{})
	go func() {
		s.tun.Close()
		err = s.instance.Close()
		close(done)
	}()
	select {
	case <-done:
		return err
	case <-time.After(FatalStopTimeout):
		os.Exit(1)
		return nil
	}
}
