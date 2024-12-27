package libbox

import (
	"context"

	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/json"
	"github.com/v2fly/v2ray-core/v5"
	"github.com/v2fly/v2ray-core/v5/common/log"
	"github.com/v2fly/v2ray-core/v5/infra/conf/cfgcommon"
	"github.com/v2fly/v2ray-core/v5/infra/conf/geodata"
	"github.com/v2fly/v2ray-core/v5/infra/conf/v5cfg"
)

func init() {
	log.RegisterHandler((*stubLogger)(nil))
}

func parseConfig(configContent string) (*core.Config, error) {
	rootConfig, err := json.UnmarshalExtended[v5cfg.RootConfig]([]byte(configContent))
	if err != nil {
		return nil, E.Cause(err, "parse config")
	}
	buildCtx := cfgcommon.NewConfigureLoadingContext(context.Background())
	cfgcommon.SetGeoDataLoader(buildCtx, common.Must1(geodata.GetGeoDataLoader("memconservative")))
	message, err := rootConfig.BuildV5(buildCtx)
	if err != nil {
		return nil, E.Cause(err, "build config")
	}
	return message.(*core.Config), nil
}

func CheckConfig(configContent string) error {
	_, err := parseConfig(configContent)
	return err
}

func FormatConfig(configContent string) (*StringBox, error) {
	return nil, E.New("format config is not supported by V2Ray")
}
